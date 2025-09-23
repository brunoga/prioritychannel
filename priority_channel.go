package prioritychannel

import (
	"fmt"
	"time"

	"github.com/brunoga/heap"
)

// New creates and returns an output channel that receives items of type T from
// the input channel `in` and sends them out in priority order, determined by
// `lessFunc`.
//
// Items read from `in` are buffered internally in a priority queue. The
// functioncontinuously sends the highest-priority item currently in the queue
// to the returned output channel. The priority is defined by the `lessFunc`,
// which should return true if item `i` has higher priority than item `j`. Note
// that the effect of prioritization is most apparent when items accumulate in
// this internal queue; if the output channel `out` is always ready to receive
// faster than items arrive on `in`, the output order may closely reflect the
// arrival order with minimal reordering.
//
// When the input channel `in` is closed, the function attempts to drain any
// remaining buffered items to the output channel. The `drainTimeout` specifies
// the maximum time to wait for the output channel to become ready for *each*
// remaining item during this drain process. If the timeout is exceeded while
// waiting to send an item, the drain process stops, and any subsequent items
// in the queue are discarded.
//
// The returned output channel (`<-chan T`) is closed when the input channel
// `in` is closed and either all buffered items have been successfully sent or
// the `drainTimeout` is exceeded during the draining phase.
//
// The processing happens in a background goroutine.
//
// The returned error wil be non-nil if invalid parameters are passed to the
// function.
func New[T any](in <-chan T, lessFunc func(i, j T) bool,
	drainTimeout time.Duration) (<-chan T, error) {
	if in == nil {
		return nil, fmt.Errorf("input channel cannot be nil")
	}
	if lessFunc == nil {
		return nil, fmt.Errorf("lessFunc cannot be nil")
	}
	if drainTimeout <= 0 {
		return nil, fmt.Errorf("drainTimeout must be greater than 0")
	}

	out := make(chan T)

	go func() {
		defer close(out)

		h := heap.NewHeap(lessFunc)

		for {
			if h.Len() == 0 {
				// The heap is empty. Wait for an item to be pushed or for the
				// channel to be closed.
				item, ok := <-in
				if !ok {
					// The input channel is closed. Return.
					return
				}

				// There is an input item. Push it to the heap.
				h.Push(item)
			}

			// Get next item from the heap.
			topItem := h.Peek()

			// The heap is not empty. Wait for an item to be pushed, for the
			// output channel to be ready or for the input channel to be closed.
			select {
			case item, ok := <-in:
				if !ok {
					for {
						// The input channel is closed. Try to send all entries
						// in the heap to the output channel.
						select {
						case out <- topItem:
							// The item was sent to the output.
							h.Pop()
						case <-time.After(drainTimeout):
							// Timeout while sending to the output channel.
							// Return.
							return
						}

						if h.Len() == 0 {
							// The heap is empty. Return.
							return
						}

						// Get next item from the heap.
						topItem = h.Peek()
					}
				}

				// There is an input item. Push it to the heap.
				h.Push(item)
			case out <- topItem:
				// The item was sent to the output.
				h.Pop()
			}
		}
	}()

	return out, nil
}
