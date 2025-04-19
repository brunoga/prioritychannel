package prioritychannel

type Heap[T any] struct {
	data     []T
	lessFunc func(i, j T) bool
}

func NewHeap[T any](lessFunc func(i, j T) bool) *Heap[T] {
	return &Heap[T]{
		data:     make([]T, 0),
		lessFunc: lessFunc,
	}
}

func (h *Heap[T]) Push(item T) {
	h.data = append(h.data, item)
	h.up(len(h.data) - 1)
}

func (h *Heap[T]) Pop() T {
	if len(h.data) == 0 {
		panic("heap is empty")
	}

	item := h.data[0]
	h.data[0] = h.data[len(h.data)-1]
	h.data = h.data[:len(h.data)-1]
	h.down(0)

	return item
}

func (h *Heap[T]) Peek() T {
	if len(h.data) == 0 {
		panic("heap is empty")
	}
	return h.data[0]
}

func (h *Heap[T]) Len() int {
	return len(h.data)
}

func (h *Heap[T]) up(i int) {
	for {
		parent := (i - 1) / 2
		if parent == i || !h.lessFunc(h.data[i], h.data[parent]) {
			break
		}
		h.data[i], h.data[parent] = h.data[parent], h.data[i]
		i = parent
	}
}

func (h *Heap[T]) down(i int) {
	for {
		left := 2*i + 1
		right := 2*i + 2
		smallest := i

		if left < len(h.data) && h.lessFunc(h.data[left], h.data[smallest]) {
			smallest = left
		}
		if right < len(h.data) && h.lessFunc(h.data[right], h.data[smallest]) {
			smallest = right
		}
		if smallest == i {
			break
		}

		h.data[i], h.data[smallest] = h.data[smallest], h.data[i]
		i = smallest
	}
}
