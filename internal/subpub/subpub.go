package subpub

import (
	"context"
	"errors"
	"sync"
)

// MessageHandler is a callback function that processes messages delivered to subscribers.
type MessageHandler func(msg interface{})

// Subscription представляет подписку, которая может быть отменена
type Subscription interface {
	// Unsubscribe will remove interest in the current subject subscription is for.
	Unsubscribe()
}

// SubPub представляет интерфейс системы публикации-подписки
type SubPub interface {
	// Subscribe создает асинхронного подписчика на указанную тему
	Subscribe(subject string, cb MessageHandler) (Subscription, error)

	// Publish публикует сообщение в указанную тему
	Publish(subject string, msg interface{}) error

	// Close завершает работу системы subpub
	// Может блокироваться до доставки данных, пока контекст не будет отменен
	Close(ctx context.Context) error
}

// Ошибки
var (
	ErrClosed = errors.New("subpub: closed")
)

// Реализация SubPub
type subPub struct {
	mu          sync.RWMutex
	subscribers map[string][]subscriber
	closed      bool
	closeCh     chan struct{}
	wg          sync.WaitGroup
}

type subscriber struct {
	ch     chan interface{}
	doneCh chan struct{}
}

type subscription struct {
	sp      *subPub
	subject string
	ch      chan interface{}
	doneCh  chan struct{}
}

// NewSubPub создает новый экземпляр SubPub
func NewSubPub() SubPub {
	return &subPub{
		subscribers: make(map[string][]subscriber),
		closeCh:     make(chan struct{}),
	}
}

func (sp *subPub) Subscribe(subject string, cb MessageHandler) (Subscription, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	if sp.closed {
		return nil, ErrClosed
	}

	ch := make(chan interface{}, 100) // Буфер для предотвращения блокировки
	doneCh := make(chan struct{})

	sub := subscriber{
		ch:     ch,
		doneCh: doneCh,
	}

	// Добавляем подписчика в мапу
	sp.subscribers[subject] = append(sp.subscribers[subject], sub)

	// Создаем объект подписки для возврата
	subscription := &subscription{
		sp:      sp,
		subject: subject,
		ch:      ch,
		doneCh:  doneCh,
	}

	// Запускаем горутину для обработки сообщений подписчика
	sp.wg.Add(1)
	go func() {
		defer sp.wg.Done()
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				cb(msg) // Вызываем обработчик сообщения
			case <-doneCh:
				return // Подписка отменена
			case <-sp.closeCh:
				return // SubPub закрыт
			}
		}
	}()

	return subscription, nil
}

func (s *subscription) Unsubscribe() {
	s.sp.mu.Lock()
	defer s.sp.mu.Unlock()

	subs := s.sp.subscribers[s.subject]
	for i, sub := range subs {
		if sub.ch == s.ch {
			// Удаляем подписчика из списка
			s.sp.subscribers[s.subject] = append(subs[:i], subs[i+1:]...)
			close(s.doneCh) // Сигнал горутине о завершении
			break
		}
	}

	// Если больше нет подписчиков на данную тему, удаляем тему из карты
	if len(s.sp.subscribers[s.subject]) == 0 {
		delete(s.sp.subscribers, s.subject)
	}
}

func (sp *subPub) Publish(subject string, msg interface{}) error {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	if sp.closed {
		return ErrClosed
	}

	subs, ok := sp.subscribers[subject]
	if !ok {
		// Если нет подписчиков, просто выходим
		return nil
	}

	// Отправляем сообщение всем подписчикам без блокировки
	for _, sub := range subs {
		select {
		case sub.ch <- msg:
			// Сообщение отправлено
		default:
			// Канал заполнен - пропускаем (медленный подписчик не должен блокировать других)
		}
	}

	return nil
}

func (sp *subPub) Close(ctx context.Context) error {
	sp.mu.Lock()
	if sp.closed {
		sp.mu.Unlock()
		return nil
	}
	sp.closed = true
	close(sp.closeCh) // Сигнал всем горутинам о завершении
	sp.mu.Unlock()

	// Ожидаем завершения всех обработчиков или отмены контекста
	done := make(chan struct{})
	go func() {
		sp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err() // Возвращаем ошибку контекста, если он отменен
	}
}
