package notifier

import (
	"context"
	"sync"
	"time"

	circuitbreaker "github.com/mhabedinpour/http-notifier/pkg/circuit-breaker"
	retryhandler "github.com/mhabedinpour/http-notifier/pkg/retry-handler"
	"github.com/mhabedinpour/http-notifier/pkg/writer"

	"github.com/eapache/channels"
	"github.com/hashicorp/go-multierror"
)

const (
	QueueNoLimit = -1
)

// Notification determines types of a message that can be used by the Queue.
type Notification = writer.Writable

// QueueOptions is options that are accepted and used by the Queue.
type QueueOptions[T Notification] struct {
	// MaxInFlightRequests determines the max allowed concurrent Writer.Write calls. Should be higher than 0.
	MaxInFlightRequests int
	// MaxQueuedNotifications is the maximum number of notifications allowed to be queued in the internal buffer. Internally a circular buffer is used, When this limit is reached, The oldest element will be removed from the queue.
	// Use QueueNoLimit to have an infinite queue. Should be higher than 0.
	MaxQueuedNotifications int
}

// Validate makes sure queue options are valid and returns error for invalid options.
func (qo QueueOptions[T]) Validate() error {
	var result error

	if qo.MaxInFlightRequests <= 0 {
		result = multierror.Append(result, ConfigurationError("MaxInFlightRequests should be higher than zero"))
	}

	if qo.MaxQueuedNotifications <= 0 && qo.MaxQueuedNotifications != QueueNoLimit {
		result = multierror.Append(result, ConfigurationError("MaxQueuedNotifications should be higher than zero or should be set to QueueNoLimit"))
	}

	return result
}

// QueueError is used for sending failed notifications to the errors channel to be read by the client.
type QueueError[T Notification] struct {
	// Notification is the Notification itself enqueued by the client.
	Notification T
	// Err specifies what is the reason of the failure.
	Err error
	// Attempts indicates how many Attempts were made for sending the Notification.
	Attempts int
}

// QueueSuccess is used for sending successful notifications to the successes channel to be read by the client.
type QueueSuccess[T Notification, R any] struct {
	// Notification is the Notification itself enqueued by the client.
	Notification T
	// Result is the result returned by the writer.
	Result R
}

// ringChannelItem is used for enqueuing notifications inside the ring channel.
type ringChannelItem[T Notification] struct {
	// notification itself
	notification T
	// attempts already made for sending the notification.
	attempts int
}

// Queue can be used to send notifications to the Writer according to the given QueueOptions.
type Queue[T Notification, R any] struct {
	// writer is used to send the queued notifications to the external entity.
	writer writer.Writer[T, R]
	// options are provided user options to control the queue behaviour.
	options QueueOptions[T]
	// ringChannel is the channel used for sending jobs to the worker goroutines. This is a ring channel, meaning if the buffer items in the channel, become higher than options.MaxQueuedNotifications, the oldest Notification will be dropped from the buffer.
	ringChannel *channels.RingChannel
	// errors channel is used to send errors to the client. Client must read from this channel, otherwise worker goroutines will get blocked.
	errors chan QueueError[T]
	// successes channel is used to notify the client that a Notification has been successfully sent. Client must read from this channel, otherwise goroutine worker goroutines will get blocked.
	successes chan QueueSuccess[T, R]
	// ctxCancelFunc is used for stopping worker goroutines.
	ctxCancelFunc context.CancelFunc
	// workersWaitGroup is used to make sure all worker goroutines are done when stopping the queue.
	workersWaitGroup sync.WaitGroup
	// retryHandlersWaitGroup is used to make sure all retry handler goroutines are done when stopping the queue.
	retryHandlersWaitGroup sync.WaitGroup
	// retryHandler can be used to retry failed notifications. This function should return how much to wait before retrying a failed Notification, Return NoRetry to stop retrying. Set this to nil to disable retrying.
	retryHandler retryhandler.RetryHandler[T]
	// circuitBreaker is used to implement the Circuit Breaker pattern and avoid cascading failures when writer is down.
	circuitBreaker circuitbreaker.CircuitBreaker[R]
}

// Errors is used to get the channel for reading errors by the client.
func (q *Queue[T, R]) Errors() <-chan QueueError[T] {
	return q.errors
}

// Successes is used by the client to get updates when a Notification has been successfully sent.
func (q *Queue[T, R]) Successes() <-chan QueueSuccess[T, R] {
	return q.successes
}

// Enqueue is used for sending new notifications.
func (q *Queue[T, R]) Enqueue(notification T) error {
	return q.sendToRingChannel(ringChannelItem[T]{
		notification: notification,
		attempts:     0,
	})
}

// sendToRingChannel tries to write the item to the ring channel, If channel is closed, it will return ErrQueueStopped.
func (q *Queue[T, R]) sendToRingChannel(item ringChannelItem[T]) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = ErrQueueStopped
		}
	}()

	q.ringChannel.In() <- item

	return err
}

// send writes enqueued notifications to the writer and retry possible errors if needed.
func (q *Queue[T, R]) send(ctx context.Context, anyItem any) {
	item, ok := anyItem.(ringChannelItem[T])
	if !ok {
		panic("invalid data type in ring channel")
	}

	item.attempts++

	result, err := q.circuitBreaker.Execute(func() (R, error) {
		return q.writer.Write(item.notification)
	})

	if err == nil {
		q.successes <- QueueSuccess[T, R]{
			Notification: item.notification,
			Result:       result,
		}

		return
	}

	waitTime := retryhandler.NoRetry
	if q.retryHandler != nil {
		waitTime = q.retryHandler.CalculateSleep(item.notification, item.attempts, err)
	}

	if waitTime == retryhandler.NoRetry {
		q.errors <- QueueError[T]{
			Notification: item.notification,
			Err:          err,
			Attempts:     item.attempts,
		}

		return
	}

	// this is safe because before waiting for the retryHandlersWaitGroup, we wait for the workersWaitGroup. It wouldn't be possible for a retryHandlersWaitGroup.Add to get executed after retryHandlersWaitGroup.wait.
	q.retryHandlersWaitGroup.Add(1)

	// wait in a separate goroutine to avoid blocking the worker, worker may be able to handle other notifications in the meantime.
	go func() {
		defer q.retryHandlersWaitGroup.Done()

		select {
		case <-time.After(waitTime):
			// enqueue again after wait time to be retried
			sendErr := q.sendToRingChannel(item)
			if sendErr != nil {
				// if ring channel was closed, stop retrying and send the last error
				q.errors <- QueueError[T]{
					Notification: item.notification,
					Err:          err,
					Attempts:     item.attempts,
				}
			}
		case <-ctx.Done():
			// if queue has been stopped during the wait, stop retrying and send the last error
			q.errors <- QueueError[T]{
				Notification: item.notification,
				Err:          err,
				Attempts:     item.attempts,
			}
		}
	}()
}

// start creates the worker goroutines which read enqueued notifications from the ring channel and send them.
func (q *Queue[T, R]) start(ctx context.Context) {
	for i := 0; i < q.options.MaxInFlightRequests; i++ {
		q.workersWaitGroup.Add(1)

		go func() {
			defer q.workersWaitGroup.Done()

			for {
				select {
				// prevents goroutine leak when queue is stopped.
				case <-ctx.Done():
					return
				case notification, ok := <-q.ringChannel.Out():
					if !ok {
						return
					}

					q.send(ctx, notification)
				}
			}
		}()
	}
}

// Stop is used to drain the queue and stop worker goroutines.
func (q *Queue[T, R]) Stop() {
	// prevents writing to the channel
	q.ringChannel.Close()

	// wait until all queued items are processed, this is not a busy-wait loop. Ring channel uses a channel internally which will block the reads when len has not changed.
	for {
		queueLen := q.ringChannel.Len()
		if queueLen == 0 {
			break
		}
	}

	// stops worker goroutines
	q.ctxCancelFunc()

	q.workersWaitGroup.Wait()       // make sure all worker goroutines are done
	q.retryHandlersWaitGroup.Wait() // make sure all retry handler goroutines are done

	close(q.errors)
	close(q.successes)
}

// NewQueue is the constructor of the Queue.
func NewQueue[T Notification, R any](writer writer.Writer[T, R], retryHandler retryhandler.RetryHandler[T], circuitBreaker circuitbreaker.CircuitBreaker[R], options QueueOptions[T]) (*Queue[T, R], error) {
	if err := options.Validate(); err != nil {
		return &Queue[T, R]{}, err
	}

	if circuitBreaker == nil {
		return &Queue[T, R]{}, ErrCircuitBreakerIsNil
	}

	ctx, ctxCancel := context.WithCancel(context.Background())
	ringChannel := channels.NewRingChannel(channels.BufferCap(options.MaxQueuedNotifications))
	queue := &Queue[T, R]{
		writer:         writer,
		options:        options,
		ringChannel:    ringChannel,
		errors:         make(chan QueueError[T], options.MaxInFlightRequests),
		successes:      make(chan QueueSuccess[T, R], options.MaxInFlightRequests),
		ctxCancelFunc:  ctxCancel,
		retryHandler:   retryHandler,
		circuitBreaker: circuitBreaker,
	}

	queue.start(ctx)

	return queue, nil
}
