package notifier

import "errors"

type ConfigurationError string

func (err ConfigurationError) Error() string {
	return "invalid configuration (" + string(err) + ")"
}

var ErrQueueStopped = errors.New("queue is stopped")
var ErrInvalidHTTPStatusCode = errors.New("HTTP status code was not valid")
