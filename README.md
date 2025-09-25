# PriorityChannel

A Go library that provides priority-based ordering for channel data delivery. Items sent to the priority channel are reordered based on a custom priority function, ensuring high-priority items are delivered first regardless of arrival order.

## Features

- **Priority-based Delivery**: Items are delivered in priority order, not arrival order
- **Generic Support**: Works with any data type using Go generics (`T any`)
- **Custom Priority Function**: Define your own priority logic with flexible `lessFunc`
- **Context Support**: Cancellable operations with full context integration
- **Graceful Shutdown**: Proper draining of pending items when input closes
- **Introspection**: Monitor pending item count with `PendingItems()` method
- **Thread-Safe**: Safe for concurrent use with proper goroutine management
- **Heap-based**: Uses efficient heap data structure for optimal performance

## Installation

```bash
go get github.com/brunoga/prioritychannel/v3
```

> **Note**: v3.0.0+ introduces breaking changes to the API. The `New()` function now returns `*PriorityChannel[T]` instead of `<-chan T`. Use the `C()` method to access the output channel and `PendingItems()` to monitor queue status.

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/brunoga/prioritychannel/v3"
)

func main() {
    // Create input channel for items
    in := make(chan int)
    
    // Create priority channel with min-heap (smallest first)
    lessFunc := func(i, j int) bool { return i < j }
    pc, err := prioritychannel.New(context.Background(), in, lessFunc)
    if err != nil {
        panic(err)
    }

    // Send items in arbitrary order
    go func() {
        defer close(in)
        
        in <- 5
        in <- 1
        in <- 3
        in <- 2
        in <- 4
    }()

    // Receive items in priority order
    for item := range pc.C() {
        fmt.Printf("Priority item: %d\n", item)
    }
    // Output: 1, 2, 3, 4, 5 (ascending order)
}
```

## API Reference

### Types

#### `PriorityChannel[T any]`

Represents a priority-based channel with methods for accessing the output and inspecting pending items.

```go
type PriorityChannel[T any] struct {
    // Internal fields - access via methods
}
```

**Methods:**
- `C() <-chan T`: Returns the output channel for receiving priority-ordered items
- `PendingItems() int`: Returns count of items in heap plus buffered input items

### Functions

#### `New[T any](ctx context.Context, in <-chan T, lessFunc func(i, j T) bool) (*PriorityChannel[T], error)`

Creates a new priority channel that reorders items from `in` based on the priority function.

**Parameters:**
- `ctx`: Context for cancellation (cannot be nil)
- `in`: Input channel for items to be prioritized (cannot be nil)
- `lessFunc`: Priority comparison function - return true if `i` has higher priority than `j` (cannot be nil)

**Returns:**
- `*PriorityChannel[T]`: Priority channel with methods for accessing output and status
- `error`: Error if invalid parameters are provided

**Behavior:**
- Items are delivered in priority order determined by `lessFunc`
- Priority effect is most apparent when items accumulate in the internal queue
- When `in` is closed, remaining items are drained in priority order
- Context cancellation closes the output channel and stops processing
- Processing happens in a background goroutine
- Access output channel via the `C()` method
- Monitor pending items via the `PendingItems()` method

## Examples

### Basic Priority Ordering (Min-Heap)

```go
in := make(chan int)
lessFunc := func(i, j int) bool { return i < j } // Smallest first
pc, _ := prioritychannel.New(context.Background(), in, lessFunc)

go func() {
    defer close(in)
    
    // These will be delivered in ascending order, not send order
    in <- 50
    in <- 10
    in <- 30
    in <- 20
}()

// Outputs: 10, 20, 30, 50
for item := range pc.C() {
    fmt.Println(item)
}
```

### Basic Priority Ordering (Max-Heap)

```go
in := make(chan int)
lessFunc := func(i, j int) bool { return i > j } // Largest first
pc, _ := prioritychannel.New(context.Background(), in, lessFunc)

go func() {
    defer close(in)
    in <- 50
    in <- 10
    in <- 30
    in <- 20
}()

// Outputs: 50, 30, 20, 10
for item := range pc.C() {
    fmt.Println(item)
}
```

### Context Cancellation

```go
ctx, cancel := context.WithCancel(context.Background())
in := make(chan int)
lessFunc := func(i, j int) bool { return i < j }
pc, _ := prioritychannel.New(ctx, in, lessFunc)

go func() {
    defer close(in)
    in <- 100
    in <- 1
}()

// Cancel processing
cancel()

// Channel closes due to cancellation
for item := range pc.C() {
    fmt.Println("May not process all items due to cancellation")
}
fmt.Println("Channel closed due to cancellation")
```

### Working with Structs

```go
type Task struct {
    ID       string
    Priority int
    Action   string
}

in := make(chan Task)
// Higher priority number = higher priority
lessFunc := func(i, j Task) bool { return i.Priority > j.Priority }
pc, _ := prioritychannel.New(context.Background(), in, lessFunc)

go func() {
    defer close(in)
    
    in <- Task{ID: "task1", Priority: 1, Action: "low priority"}
    in <- Task{ID: "task2", Priority: 5, Action: "high priority"}
    in <- Task{ID: "task3", Priority: 3, Action: "medium priority"}
}()

for task := range pc.C() {
    fmt.Printf("Executing task %s (priority %d): %s\n", 
        task.ID, task.Priority, task.Action)
}
// Output: task2 (priority 5), task3 (priority 3), task1 (priority 1)
```

### Monitoring Pending Items

```go
in := make(chan string, 10) // Buffered channel
lessFunc := func(i, j string) bool { return i < j }
pc, _ := prioritychannel.New(context.Background(), in, lessFunc)

// Add items to buffer
in <- "zebra"
in <- "apple"
in <- "banana"

// Check pending items (includes items in heap + buffered input)
fmt.Printf("Pending items: %d\n", pc.PendingItems())

close(in)
// Process items
for item := range pc.C() {
    fmt.Printf("Processing: %s, Remaining: %d\n", item, pc.PendingItems())
}
// Output shows items processed in alphabetical order
```

### Timeout Context

```go
// Automatically cancel after 5 seconds
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

in := make(chan int)
lessFunc := func(i, j int) bool { return i < j }
pc, _ := prioritychannel.New(ctx, in, lessFunc)

// Send items...
```

## Implementation Details

- **Priority Queue**: Uses [brunoga/heap](https://github.com/brunoga/heap) for efficient priority-based ordering
- **Goroutine Management**: Single background goroutine per priority channel
- **Memory Efficiency**: Items are stored in heap until they can be sent to output
- **Thread Safety**: Safe for concurrent producers sending to input channel
- **Algorithm**: O(log n) insertion, O(log n) extraction for optimal performance

## Performance Considerations

- **Time Complexity**: O(log n) for insertion, O(log n) for extraction
- **Memory Usage**: O(n) where n is the number of pending items
- **Goroutines**: One background goroutine per priority channel instance
- **Channel Buffering**: Uses unbuffered output channel; input can be buffered
- **Priority Effect**: Most beneficial when items accumulate (slow consumer scenarios)

## Testing

The library includes comprehensive tests with both real-time and deterministic testing using Go's `testing/synctest` package for reliable concurrent testing.

```bash
# Run all tests
go test -v

# Run specific test
go test -v -run TestPriorityChannelBasicOrderMin
```

## Requirements

- Go 1.25.1 or later (uses generics)
- [brunoga/heap](https://github.com/brunoga/heap) v1.0.0

## License

See LICENSE file.

## Contributing

Contributions are welcome. Feel free to send pull requests.

## Related Projects

- [schedulechannel](https://github.com/brunoga/schedulechannel) - Time-based scheduling for channels

---

For more examples and advanced usage, see the test files in this repository.
