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

// processItems handles the core processing logic for the priority channel.
// It processes items from the input channel, maintains them in priority order
// in the heap, and sends them to the output channel.
func (pc *PriorityChannel[T]) processItems(ctx context.Context, in <-chan T, out chan<- T) {
	defer close(out)

	// This flag tracks if the input channel is closed,
	// so we know when to start draining.
	var inputClosed bool

	for {
		// --- 1. Define channels for the select ---

		// 'outChan' is the channel we send to.
		// It starts as nil (disabled) by default.
		var outChan chan<- T
		var topItem T

		// If the heap has items, we enable the 'outChan' case
		// by pointing it to the real 'out' channel and
		// peeking at the next item to send.
		if pc.h.Len() > 0 {
			topItem = pc.h.Peek()
			outChan = out // Enable the send case
		}

		// 'inChan' is the channel we receive from.
		// If 'inputClosed' is true, we set 'inChan' to nil
		// to disable the receive case.
		var inChan <-chan T = in
		if inputClosed {
			inChan = nil // Disable the receive case
		}

		// --- 2. Check for exit condition ---

		// If the input is closed AND the heap is empty,
		// we are done draining and can exit.
		if inputClosed && pc.h.Len() == 0 {
			return
		}

		// --- 3. Dynamically select on active channels ---

		select {
		case <-ctx.Done():
			// Context was cancelled, exit immediately
			return

		case item, ok := <-inChan: // Case is disabled if inChan is nil
			if !ok {
				// Input channel is now closed. Set the flag.
				// The loop will repeat, and 'inChan' will be nil
				// from now on, effectively starting the drain.
				inputClosed = true
			} else {
				// Got a new item, add it to the heap.
				pc.h.Push(item)
			}

		case outChan <- topItem: // Case is disabled if outChan is nil
			// We successfully sent the highest-priority item.
			// Pop it from the heap.
			pc.h.Pop()
		}
	}
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

	go pc.processItems(ctx, in, out)

	return pc, nil
}
