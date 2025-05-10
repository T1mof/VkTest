package subpub

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSubscribeAndPublish(t *testing.T) {
	sp := NewSubPub()

	var received []string
	var mu sync.Mutex

	sub, err := sp.Subscribe("test-subject", func(msg interface{}) {
		mu.Lock()
		received = append(received, msg.(string))
		mu.Unlock()
	})

	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Публикуем сообщения
	err = sp.Publish("test-subject", "message1")
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	err = sp.Publish("test-subject", "message2")
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Даем время на обработку сообщений
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if len(received) != 2 || received[0] != "message1" || received[1] != "message2" {
		t.Fatalf("Expected [message1, message2], got %v", received)
	}
	mu.Unlock()

	// Отписываемся
	sub.Unsubscribe()

	// Публикуем еще одно сообщение, которое не должно быть получено
	err = sp.Publish("test-subject", "message3")
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if len(received) != 2 {
		t.Fatalf("Expected 2 messages after unsubscribe, got %d", len(received))
	}
	mu.Unlock()

	// Закрываем
	err = sp.Close(context.Background())
	if err != nil {
		t.Fatalf("Failed to close: %v", err)
	}
}

func TestMultipleSubscribers(t *testing.T) {
	sp := NewSubPub()
	var receivedCount1, receivedCount2 int
	var mu sync.Mutex

	// Подписываем двух подписчиков на один subject
	sub1, err := sp.Subscribe("test-subject", func(msg interface{}) {
		mu.Lock()
		receivedCount1++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to subscribe first subscriber: %v", err)
	}

	sub2, err := sp.Subscribe("test-subject", func(msg interface{}) {
		mu.Lock()
		receivedCount2++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Failed to subscribe second subscriber: %v", err)
	}

	// Публикуем сообщение
	if err := sp.Publish("test-subject", "message"); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Проверяем, что оба подписчика получили сообщение
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	if receivedCount1 != 1 || receivedCount2 != 1 {
		t.Fatalf("Expected both subscribers to receive the message")
	}
	mu.Unlock()

	// Проверяем функциональность отписки для уверенности
	sub1.Unsubscribe()
	sub2.Unsubscribe()

	// Закрываем
	if err := sp.Close(context.Background()); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}
}

func TestSlowSubscriber(t *testing.T) {
	sp := NewSubPub()

	var slowReceived, fastReceived []string
	var slow, fast sync.Mutex

	// Медленный подписчик
	_, err := sp.Subscribe("test-subject", func(msg interface{}) {
		slow.Lock()
		slowReceived = append(slowReceived, msg.(string))
		slow.Unlock()
		time.Sleep(50 * time.Millisecond) // Симуляция медленной обработки
	})

	if err != nil {
		t.Fatalf("Failed to subscribe slow: %v", err)
	}

	// Быстрый подписчик
	_, err = sp.Subscribe("test-subject", func(msg interface{}) {
		fast.Lock()
		fastReceived = append(fastReceived, msg.(string))
		fast.Unlock()
	})

	if err != nil {
		t.Fatalf("Failed to subscribe fast: %v", err)
	}

	// Публикуем много сообщений
	for i := 0; i < 20; i++ {
		err = sp.Publish("test-subject", "message")
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
	}

	// Даем время на обработку сообщений
	time.Sleep(200 * time.Millisecond)

	fast.Lock()
	fastCount := len(fastReceived)
	fast.Unlock()

	// Проверяем, что быстрый подписчик получил все сообщения
	if fastCount != 20 {
		t.Fatalf("Fast subscriber expected to receive 20 messages, got %d", fastCount)
	}

	// Закрываем
	if err := sp.Close(context.Background()); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}
}

func TestCloseWithContext(t *testing.T) {
	sp := NewSubPub()

	_, err := sp.Subscribe("test-subject", func(msg interface{}) {
		time.Sleep(300 * time.Millisecond) // Длительная обработка
	})
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Публикуем сообщение
	if err := sp.Publish("test-subject", "message"); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Создаем контекст с таймаутом меньше времени обработки
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Close должен вернуться после таймаута
	start := time.Now()
	err = sp.Close(ctx)
	elapsed := time.Since(start)

	if elapsed > 200*time.Millisecond {
		t.Fatalf("Close took too long, should have returned after context timeout")
	}

	// Проверяем, что ошибка связана с контекстом
	if err == nil || err.Error() != ctx.Err().Error() {
		t.Fatalf("Expected context error (%v), got %v", ctx.Err(), err)
	}
}
