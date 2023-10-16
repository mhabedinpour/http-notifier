package writer

import "io"

type Writable = io.Reader

// Writer interface specifies a client of an external entity (e.g.: message broker, http api, file, tcp socket, etc.) which will send the queued notifications to the external entity.
type Writer[T Writable, R any] interface {
	// Write is called by the Queue to send notifications to the external entity.
	Write(notification T) (R, error)
}
