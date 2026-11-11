package workers

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type TaskEventType string

const (
	TaskEventCreated   TaskEventType = "TASK_CREATED"
	TaskEventUpdated   TaskEventType = "TASK_UPDATED"
	TaskEventCompleted TaskEventType = "TASK_COMPLETED"
)

// TaskEvent represents an asynchronous message published when task lifecycle changes happen.
type TaskEvent struct {
	TaskID    int64
	UserID    int64
	Title     string
	EventType TaskEventType
	Timestamp time.Time
}

// TaskEventWorkerPool manages a fixed pool of background workers consuming task events.
type TaskEventWorkerPool struct {
	eventsQueue chan TaskEvent
	waitGroup   sync.WaitGroup
	numWorkers  int
	stopOnce    sync.Once
}

// NewTaskEventWorkerPool initializes a worker pool with a buffered channel.
func NewTaskEventWorkerPool(bufferSize int, numWorkers int) *TaskEventWorkerPool {
	return &TaskEventWorkerPool{
		eventsQueue: make(chan TaskEvent, bufferSize),
		numWorkers:  numWorkers,
	}
}

// Start spawns the fixed number of worker goroutines.
func (pool *TaskEventWorkerPool) Start(ctx context.Context) {
	for workerID := 1; workerID <= pool.numWorkers; workerID++ {
		pool.waitGroup.Add(1)
		go pool.runWorker(workerID, ctx)
	}
	log.Printf("[Worker Pool] Started %d background task event workers\n", pool.numWorkers)
}

// Enqueue sends an event to the queue using a non-blocking select.
// If the buffer is full, it drops the event rather than stalling the HTTP caller.
func (pool *TaskEventWorkerPool) Enqueue(event TaskEvent) bool {
	select {
	case pool.eventsQueue <- event:
		return true
	default:
		log.Printf("[Worker Pool] Warning: event queue full, dropped %s event for task %d\n", event.EventType, event.TaskID)
		return false
	}
}

// Stop gracefully shuts down the worker pool by closing the channel and waiting for workers to finish.
func (pool *TaskEventWorkerPool) Stop() {
	pool.stopOnce.Do(func() {
		close(pool.eventsQueue)
		pool.waitGroup.Wait()
		log.Println("[Worker Pool] All background task workers stopped cleanly")
	})
}

func (pool *TaskEventWorkerPool) runWorker(workerID int, ctx context.Context) {
	defer pool.waitGroup.Done()

	for {
		select {
		case event, ok := <-pool.eventsQueue:
			if !ok {
				// Channel closed, worker drains and exits
				return
			}
			pool.processEvent(workerID, event)
		case <-ctx.Done():
			// Context canceled, finish remaining
			return
		}
	}
}

// processEvent simulates async background actions like audit logging or email notifications.
func (pool *TaskEventWorkerPool) processEvent(workerID int, event TaskEvent) {
	time.Sleep(50 * time.Millisecond) // simulate I/O (e.g., SMTP or external webhook)

	switch event.EventType {
	case TaskEventCreated:
		fmt.Printf("[Worker %d] AUDIT: Task #%d '%s' created for user #%d at %s\n",
			workerID, event.TaskID, event.Title, event.UserID, event.Timestamp.Format(time.RFC3339))
	case TaskEventCompleted:
		fmt.Printf("[Worker %d] NOTIFICATION: Task #%d '%s' marked completed! Sent email alert to user #%d\n",
			workerID, event.TaskID, event.Title, event.UserID)
	default:
		fmt.Printf("[Worker %d] AUDIT: Task #%d '%s' updated (event: %s)\n",
			workerID, event.TaskID, event.Title, event.EventType)
	}
}
