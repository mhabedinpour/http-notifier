package notifier

import "errors"

type ConfigurationError string

func (err ConfigurationError) Error() string {
	return "invalid configuration (" + string(err) + ")"
}

var ErrQueueStopped = errors.New("queue is stopped")
var ErrCircuitBreakerIsNil = errors.New("circuit breaker is nil")
