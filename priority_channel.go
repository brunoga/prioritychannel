package prioritychannel

import (
	"context"
	"fmt"

	"github.com/brunoga/heap"
)

// PriorityChannel represents a priority-based channel.
type PriorityChannel[T any] struct {
	c  <-chan T      // Output channel that receives items in priority order
	in <-chan T      // Input channel reference for introspection
	h  *heap.Heap[T] // Internal priority heap
}

// C returns the output channel that receives items in priority order.
func (pc *PriorityChannel[T]) C() <-chan T {
	return pc.c
}

// PendingItems returns the total number of items currently pending:
// items in the internal priority heap plus items buffered in the input channel.
// For unbuffered input channels, this will only count items in the heap.
func (pc *PriorityChannel[T]) PendingItems() int {
	heapLen := pc.h.Len()
	inputLen := len(pc.in)
	return heapLen + inputLen
}

// New creates and returns a PriorityChannel that receives items of type T from
// the input channel `in` and sends them out in priority order, determined by
// `lessFunc`.
//
// Items read from `in` are buffered internally in a priority queue. The
// function continuously sends the highest-priority item currently in the queue
// to the returned PriorityChannel's output channel (accessed via the C() method).
// The priority is defined by the `lessFunc`, which should return true if item
// `i` has higher priority than item `j`. Note that the effect of prioritization
// is most apparent when items accumulate in this internal queue; if the output
// channel is always ready to receive faster than items arrive on `in`, the
// output order may closely reflect the arrival order with minimal reordering.
//
// When the input channel `in` is closed, the function attempts to drain any
// remaining buffered items to the output channel. The context can be used to
// control timeouts and cancellation. If the context is cancelled or times out
// while waiting to send an item during the drain process, the drain stops and
// any remaining items in the queue are discarded.
//
// The returned PriorityChannel's C member (`<-chan T`) is closed when:
// - The input channel `in` is closed and all buffered items have been sent, or
// - The context is cancelled or times out during processing
//
// The processing happens in a background goroutine.
//
// The returned error will be non-nil if invalid parameters are passed to the
// function.
func New[T any](ctx context.Context, in <-chan T, lessFunc func(i, j T) bool) (*PriorityChannel[T], error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}
	if in == nil {
		return nil, fmt.Errorf("input channel cannot be nil")
	}
	if lessFunc == nil {
		return nil, fmt.Errorf("lessFunc cannot be nil")
	}

	out := make(chan T)
	h := heap.NewHeap(lessFunc)

	pc := &PriorityChannel[T]{
		c:  out,
		in: in,
		h:  h,
	}

	go func() {
		defer close(out)

		for {
			if h.Len() == 0 {
				// The heap is empty. Wait for an item to be pushed, for the
				// channel to be closed, or for context cancellation.
				select {
				case item, ok := <-in:
					if !ok {
						// The input channel is closed. Return.
						return
					}
					// There is an input item. Push it to the heap.
					h.Push(item)
				case <-ctx.Done():
					// Context cancelled, return immediately
					return
				}
			} else {
				// Get next item from the heap.
				topItem := h.Peek()

				// The heap is not empty. Wait for an item to be pushed, for the
				// output channel to be ready, for the input channel to be closed,
				// or for context cancellation.
				select {
				case item, ok := <-in:
					if !ok {
						// Input channel closed, try to drain remaining items
						for h.Len() > 0 {
							topItem := h.Peek()
							select {
							case out <- topItem:
								// The item was sent to the output.
								h.Pop()
							case <-ctx.Done():
								// Context cancelled during drain, drop remaining items
								return
							}
						}
						// All items drained successfully
						return
					}
					// There is an input item. Push it to the heap.
					h.Push(item)
				case out <- topItem:
					// The item was sent to the output.
					h.Pop()
				case <-ctx.Done():
					// Context cancelled, return immediately
					return
				}
			}
		}
	}()

	return pc, nil
}
