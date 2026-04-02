package main

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSSEHub_SubscribeUnsubscribe(t *testing.T) {
	hub := NewSSEHub()
	ch := hub.Subscribe()

	hub.mu.RLock()
	count := len(hub.clients)
	hub.mu.RUnlock()
	if count != 1 {
		t.Errorf("client count after subscribe = %d, want 1", count)
	}

	hub.Unsubscribe(ch)

	hub.mu.RLock()
	count = len(hub.clients)
	hub.mu.RUnlock()
	if count != 0 {
		t.Errorf("client count after unsubscribe = %d, want 0", count)
	}
}

func TestSSEHub_UnsubscribeTwice(t *testing.T) {
	hub := NewSSEHub()
	ch := hub.Subscribe()
	hub.Unsubscribe(ch)
	// Second unsubscribe should be a no-op, not panic
	hub.Unsubscribe(ch)
}

func TestSSEHub_BroadcastFormat(t *testing.T) {
	hub := NewSSEHub()
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	hub.Broadcast("refresh", "tasks")

	select {
	case msg := <-ch:
		if !strings.Contains(msg, "event: refresh") {
			t.Errorf("message missing event field: %q", msg)
		}
		if !strings.Contains(msg, "data: tasks") {
			t.Errorf("message missing data field: %q", msg)
		}
		// SSE messages must end with double newline
		if !strings.HasSuffix(msg, "\n\n") {
			t.Errorf("message should end with \\n\\n: %q", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for broadcast")
	}
}

func TestSSEHub_BroadcastToMultiple(t *testing.T) {
	hub := NewSSEHub()
	ch1 := hub.Subscribe()
	ch2 := hub.Subscribe()
	ch3 := hub.Subscribe()
	defer hub.Unsubscribe(ch1)
	defer hub.Unsubscribe(ch2)
	defer hub.Unsubscribe(ch3)

	hub.Broadcast("update", "data")

	for i, ch := range []chan string{ch1, ch2, ch3} {
		select {
		case msg := <-ch:
			if !strings.Contains(msg, "data: data") {
				t.Errorf("client %d got unexpected message: %q", i, msg)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("client %d timed out", i)
		}
	}
}

func TestSSEHub_BroadcastSkipsFullChannel(t *testing.T) {
	hub := NewSSEHub()
	ch := hub.Subscribe()
	defer hub.Unsubscribe(ch)

	// Fill the channel buffer (capacity 16)
	for i := 0; i < 16; i++ {
		hub.Broadcast("fill", "x")
	}

	// This should not block even though channel is full
	done := make(chan struct{})
	go func() {
		hub.Broadcast("overflow", "y")
		close(done)
	}()

	select {
	case <-done:
		// good — broadcast didn't block
	case <-time.After(1 * time.Second):
		t.Fatal("broadcast blocked on full channel")
	}
}

func TestSSEHub_ConcurrentAccess(t *testing.T) {
	hub := NewSSEHub()
	var wg sync.WaitGroup

	// Concurrent subscribes, broadcasts, and unsubscribes
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := hub.Subscribe()
			hub.Broadcast("test", "concurrent")
			hub.Unsubscribe(ch)
		}()
	}

	wg.Wait()

	hub.mu.RLock()
	count := len(hub.clients)
	hub.mu.RUnlock()
	if count != 0 {
		t.Errorf("all clients should be cleaned up, got %d remaining", count)
	}
}
