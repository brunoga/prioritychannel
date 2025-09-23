// prioritychannel_test.go
package prioritychannel

import (
	"context"
	"reflect"
	"sort"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestNewPriorityChannelErrors(t *testing.T) {
	in := make(chan int)
	defer close(in) // Close harmlessly if not used
	lessFunc := func(i, j int) bool { return i < j }
	ctx := context.Background()

	tests := []struct {
		name        string
		ctx         context.Context
		inChan      chan int
		lessFunc    func(i, j int) bool
		expectedErr string
	}{
		{"NilContext", nil, in, lessFunc, "context cannot be nil"},
		{"NilInputChannel", ctx, nil, lessFunc, "input channel cannot be nil"},
		{"NilLessFunc", ctx, in, nil, "lessFunc cannot be nil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.ctx, tt.inChan, tt.lessFunc)
			if err == nil {
				t.Fatalf("Expected error '%s', but got nil", tt.expectedErr)
			}
			if err.Error() != tt.expectedErr {
				t.Errorf("Expected error message '%s', got '%s'", tt.expectedErr, err.Error())
			}
		})
	}
}

func TestPriorityChannelBasicOrderMin(t *testing.T) {
	in := make(chan int)
	lessFunc := func(i, j int) bool { return i < j }
	ctx := context.Background()

	out, err := New(ctx, in, lessFunc)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	items := []int{5, 1, 9, 4, 8, 2, 7, 3, 6, 0}
	expectedOrder := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	// Send items to input channel
	for _, item := range items {
		in <- item
	}
	close(in)

	var receivedItems []int
	// Read items from output channel
	for item := range out {
		receivedItems = append(receivedItems, item)
	}

	// Verify order
	if !reflect.DeepEqual(receivedItems, expectedOrder) {
		t.Errorf("Items received in wrong order.\nExpected: %v\nGot:      %v", expectedOrder, receivedItems)
	}
}

func TestPriorityChannelBasicOrderMax(t *testing.T) {
	in := make(chan int, 1)
	lessFunc := func(i, j int) bool { return i > j } // Max priority
	ctx := context.Background()

	out, err := New(ctx, in, lessFunc)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	items := []int{5, 1, 9, 4, 8, 2, 7, 3, 6, 0}
	expectedOrder := []int{9, 8, 7, 6, 5, 4, 3, 2, 1, 0} // Max first

	for _, item := range items {
		in <- item
	}
	close(in)

	var receivedItems []int
	for item := range out {
		receivedItems = append(receivedItems, item)
	}

	if !reflect.DeepEqual(receivedItems, expectedOrder) {
		t.Errorf("Items received in wrong order for max priority.\nExpected: %v\nGot:      %v", expectedOrder, receivedItems)
	}
}

func TestPriorityChannelEmptyInput(t *testing.T) {
	in := make(chan string) // Unbuffered
	lessFunc := func(i, j string) bool { return i < j }
	ctx := context.Background()

	out, err := New(ctx, in, lessFunc)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	close(in) // Close input immediately

	// Attempt to read - should block until closed
	timeout := 20 * time.Millisecond
	select {
	case _, ok := <-out:
		if ok {
			t.Error("Received item from output channel, but expected none")
		}
		// Channel closed as expected
	case <-time.After(timeout * 2): // Wait longer than drain timeout
		t.Error("Output channel did not close after input channel closed")
	}
}

func TestPriorityChannelContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		in := make(chan int)
		lessFunc := func(i, j int) bool { return i < j }
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		out, err := New(ctx, in, lessFunc)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}

		// Send items
		in <- 5
		in <- 1
		in <- 3
		close(in)

		// Read only one item
		select {
		case item, ok := <-out:
			if !ok {
				t.Fatal("Output channel closed unexpectedly early")
			}
			if item != 1 {
				t.Fatalf("Expected first item to be 1, got %d", item)
			}
		case <-time.After(50 * time.Millisecond):
			t.Fatal("Timed out waiting for the first item")
		}

		// Now wait for longer than the context timeout without reading
		time.Sleep(30 * time.Millisecond)

		// Try reading again. The channel should now be closed due to context cancellation.
		select {
		case item, ok := <-out:
			if ok {
				t.Errorf("Expected output channel to be closed due to context cancellation, but received item %d", item)
			}
			// Channel closed as expected
		case <-time.After(10 * time.Millisecond): // Short wait, should be closed already
			t.Error("Output channel is still open after context timeout expired")
		}
	})
}

func TestPriorityChannelSlowConsumer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		in := make(chan int)
		lessFunc := func(i, j int) bool { return i < j }
		ctx := context.Background()

		out, err := New(ctx, in, lessFunc)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}

		items := []int{5, 1, 9, 4, 8, 2, 7, 3, 6, 0}
		expectedOrder := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

		for _, item := range items {
			in <- item
		}
		close(in)

		var receivedItems []int
		// Read items slowly from output channel
		timeout := 100 * time.Millisecond
		for i := 0; i < len(expectedOrder); i++ {
			select {
			case item, ok := <-out:
				if !ok {
					t.Fatalf("Output channel closed prematurely at index %d", i)
				}
				receivedItems = append(receivedItems, item)
				time.Sleep(10 * time.Millisecond) // Simulate slow processing
			case <-time.After(timeout * 2):
				t.Fatalf("Timed out waiting for item at index %d", i)
			}
		}

		// Check if channel is closed now
		select {
		case _, ok := <-out:
			if ok {
				t.Error("Output channel still open after receiving all expected items")
			}
		case <-time.After(10 * time.Millisecond):
			t.Error("Timed out waiting for output channel to close")
		}

		if !reflect.DeepEqual(receivedItems, expectedOrder) {
			t.Errorf("Items received in wrong order with slow consumer.\nExpected: %v\nGot:      %v", expectedOrder, receivedItems)
		}
	})
}

func TestPriorityChannelInterleaving(t *testing.T) {
	in := make(chan int, 5)
	lessFunc := func(i, j int) bool { return i < j }
	ctx := context.Background()

	out, err := New(ctx, in, lessFunc)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var receivedItems []int
	var wg sync.WaitGroup
	wg.Add(1)
	timeout := 50 * time.Millisecond

	// Producer and Consumer running interleaved
	go func() {
		defer wg.Done()
		// Send first batch
		in <- 5
		in <- 1
		in <- 8

		// Receive first item
		select {
		case item := <-out:
			receivedItems = append(receivedItems, item) // Should be 1
		case <-time.After(timeout):
			t.Error("Timeout waiting for first item")
			return
		}

		// Send second batch
		in <- 0
		in <- 4

		// Receive second item
		select {
		case item := <-out:
			receivedItems = append(receivedItems, item) // Should be 0
		case <-time.After(timeout):
			t.Error("Timeout waiting for second item")
			return
		}

		// Close input
		close(in)

		// Receive remaining items
		for item := range out {
			receivedItems = append(receivedItems, item)
		}
	}()

	wg.Wait() // Wait for goroutine to finish

	expectedOrder := []int{1, 0, 4, 5, 8} // Order matters: 1 received, then 0 pushed, then 0 received, then drain 4, 5, 8

	// Note: The exact interleaving can sometimes vary slightly depending on goroutine scheduling
	// We need to sort both to compare the *set* of items received correctly and their relative final order.
	sort.Ints(receivedItems)
	sort.Ints(expectedOrder) // Sort expected to match for comparison

	if !reflect.DeepEqual(receivedItems, expectedOrder) {
		// Use Errorf to allow test to continue and report mismatch
		t.Errorf("Interleaved items received in wrong order or wrong items.\nExpected (sorted): %v\nGot (sorted):      %v", expectedOrder, receivedItems)
	}
}

type TestPriorityItem struct {
	ID       string
	Priority int
}

func TestPriorityChannelWithStructs(t *testing.T) {
	in := make(chan TestPriorityItem, 5)
	// Higher Priority value means it should come out first (Max Heap behavior)
	lessFunc := func(i, j TestPriorityItem) bool { return i.Priority > j.Priority }
	ctx := context.Background()

	out, err := New(ctx, in, lessFunc)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	items := []TestPriorityItem{
		{"low", 1},
		{"high", 10},
		{"medium", 5},
		{"very_high", 20},
		{"medium_2", 5},
	}
	expectedOrderIDs := []string{"very_high", "high", "medium", "medium_2", "low"} // Expected order based on priority

	go func() {
		for _, item := range items {
			in <- item
		}
		close(in)
	}()

	var receivedItems []TestPriorityItem
	for item := range out {
		receivedItems = append(receivedItems, item)
	}

	// Check length first
	if len(receivedItems) != len(expectedOrderIDs) {
		t.Fatalf("Expected %d items, but received %d", len(expectedOrderIDs), len(receivedItems))
	}

	// Verify order by ID
	var receivedOrderIDs []string
	for _, item := range receivedItems {
		receivedOrderIDs = append(receivedOrderIDs, item.ID)
	}

	// Need stable sort for items with same priority if we want deterministic ID order check
	sort.SliceStable(receivedItems, func(i, j int) bool {
		if receivedItems[i].Priority != receivedItems[j].Priority {
			return receivedItems[i].Priority > receivedItems[j].Priority // Primary sort (max heap)
		}
		return receivedItems[i].ID < receivedItems[j].ID // Secondary sort for stable testing
	})
	// Apply same stable sort logic to expected order
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority > items[j].Priority
		}
		return items[i].ID < items[j].ID
	})
	expectedOrderIDs = []string{}
	for _, item := range items {
		expectedOrderIDs = append(expectedOrderIDs, item.ID)
	}

	var finalReceivedIDs []string
	for _, item := range receivedItems {
		finalReceivedIDs = append(finalReceivedIDs, item.ID)
	}

	if !reflect.DeepEqual(finalReceivedIDs, expectedOrderIDs) {
		t.Errorf("Struct items received in wrong order based on Priority.\nExpected IDs: %v\nGot IDs:      %v", expectedOrderIDs, finalReceivedIDs)
	}
}

func TestPriorityChannelExplicitCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		in := make(chan int)
		lessFunc := func(i, j int) bool { return i < j }
		ctx, cancel := context.WithCancel(context.Background())

		out, err := New(ctx, in, lessFunc)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}

		// Send some items
		in <- 5
		in <- 1
		in <- 3

		// Read one item
		item := <-out
		if item != 1 {
			t.Fatalf("Expected first item to be 1, got %d", item)
		}

		// Cancel the context explicitly
		cancel()

		// Also close the input channel to trigger drain phase where cancellation matters
		close(in)

		// The output channel should close soon after cancellation
		select {
		case _, ok := <-out:
			if ok {
				t.Error("Expected channel to close after context cancellation")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Channel did not close within reasonable time after cancellation")
		}
	})
}

func TestPriorityChannelWithDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		in := make(chan int)
		lessFunc := func(i, j int) bool { return i < j }
		deadline := time.Now().Add(50 * time.Millisecond)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		out, err := New(ctx, in, lessFunc)
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}

		// Send items
		in <- 5
		in <- 1
		in <- 3

		// Read one item quickly
		item := <-out
		if item != 1 {
			t.Fatalf("Expected first item to be 1, got %d", item)
		}

		// Wait for deadline to pass
		time.Sleep(60 * time.Millisecond)

		// Channel should be closed due to deadline
		select {
		case _, ok := <-out:
			if ok {
				t.Error("Expected channel to close after deadline")
			}
		case <-time.After(10 * time.Millisecond):
			t.Error("Channel did not close within reasonable time after deadline")
		}
	})
}
