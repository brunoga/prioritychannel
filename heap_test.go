package prioritychannel

import (
	"testing"
)

func TestHeap_PushPop(t *testing.T) {
	h := NewHeap(func(i, j int) bool { return i < j })

	h.Push(3)
	h.Push(1)
	h.Push(2)

	if h.Len() != 3 {
		t.Fatalf("expected length 3, got %d", h.Len())
	}

	if item := h.Pop(); item != 1 {
		t.Fatalf("expected 1, got %d", item)
	}

	if item := h.Pop(); item != 2 {
		t.Fatalf("expected 2, got %d", item)
	}

	if item := h.Pop(); item != 3 {
		t.Fatalf("expected 3, got %d", item)
	}

	if h.Len() != 0 {
		t.Fatalf("expected length 0, got %d", h.Len())
	}
}

func TestHeap_Peek(t *testing.T) {
	h := NewHeap(func(i, j int) bool { return i < j })

	h.Push(3)
	h.Push(1)
	h.Push(2)

	if item := h.Peek(); item != 1 {
		t.Fatalf("expected 1, got %d", item)
	}

	if h.Len() != 3 {
		t.Fatalf("expected length 3, got %d", h.Len())
	}
}

func TestHeap_EmptyPop(t *testing.T) {
	h := NewHeap(func(i, j int) bool { return i < j })

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on empty pop")
		}
	}()

	h.Pop()
}

func TestHeap_EmptyPeek(t *testing.T) {
	h := NewHeap(func(i, j int) bool { return i < j })

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on empty peek")
		}
	}()

	h.Peek()
}
