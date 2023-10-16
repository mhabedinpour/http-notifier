package retryhandler

import (
	"time"
)

const NoRetry = time.Duration(0)

// RetryHandler is used to retry failures in notification sending.
type RetryHandler[T any] interface {
	// CalculateSleep is used to calculate how much to wait before retrying a failed Notification, Return NoRetry to stop retrying.
	CalculateSleep(notification T, attempts int, err error) time.Duration
}
