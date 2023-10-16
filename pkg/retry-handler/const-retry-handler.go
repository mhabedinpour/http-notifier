package retryhandler

import (
	"time"
)

// ConstRetryHandler is a RetryHandler which will retry a notification for MaxAttempts and waits WaitTime between each attempt.
type ConstRetryHandler[T any] struct {
	// WaitTime determines how much to wait between each attempt.
	WaitTime time.Duration
	// MaxAttempts is the maximum number of attempts to make before considering notification sending as failure.
	MaxAttempts int
}

// to make sure ConstRetryHandler implements RetryHandler.
var _ RetryHandler[any] = (*ConstRetryHandler[any])(nil)

func (c ConstRetryHandler[T]) CalculateSleep(_ T, attempts int, _ error) time.Duration {
	if attempts >= c.MaxAttempts {
		return NoRetry
	}

	return c.WaitTime
}

// NewConstRetryHandler creates a new ConstRetryHandler.
func NewConstRetryHandler[T any](waitTime time.Duration, maxAttempts int) ConstRetryHandler[T] {
	return ConstRetryHandler[T]{
		WaitTime:    waitTime,
		MaxAttempts: maxAttempts,
	}
}
