package notifier

import (
	"net/http"
	"time"
)

// Writer interface specifies a client of an external entity (e.g.: message broker, http api, file, tcp socket, etc.) which will send the queued notifications to the external entity.
type Writer[T Notification] interface {
	// Write is called by the Queue to send notifications to the external entity.
	Write(notification T) error
}

// HTTPPostWriter is an implementation of Writer which sends notifications by making HTTP POST calls to a given url.
type HTTPPostWriter[T Notification] struct {
	// url is the url to which HTTP POST calls are made.
	url string
	// client is used for sending http requests.
	client *http.Client
}

// to make sure HTTPPostWriter implements Writer.
var _ Writer[Notification] = (*HTTPPostWriter[Notification])(nil)

func (h *HTTPPostWriter[T]) Write(notification T) error {
	req, err := http.NewRequest("POST", h.url, notification)
	if err != nil {
		return err
	}

	res, err := h.client.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if statusOK := res.StatusCode >= 200 && res.StatusCode < 300; !statusOK { // nolint: gomnd
		return ErrInvalidHTTPStatusCode
	}

	return nil
}

// NewHTTPPostWriter is the constructor of NewHTTPPostWriter.
func NewHTTPPostWriter[T Notification](url string, timeout time.Duration) *HTTPPostWriter[T] {
	return &HTTPPostWriter[T]{
		url: url,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}
