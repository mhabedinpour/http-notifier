package writer

import (
	"errors"
	"io"
	"net/http"
	"time"
)

var ErrInvalidHTTPStatusCode = errors.New("HTTP status code was not valid")

// HTTPPostWriter is an implementation of Writer which sends notifications by making HTTP POST calls to a given url.
type HTTPPostWriter[T Writable] struct {
	// url is the url to which HTTP POST calls are made.
	url string
	// client is used for sending http requests.
	client *http.Client
}

// to make sure HTTPPostWriter implements Writer.
var _ Writer[Writable, []byte] = (*HTTPPostWriter[Writable])(nil)

func (h *HTTPPostWriter[T]) Write(notification T) ([]byte, error) {
	req, err := http.NewRequest("POST", h.url, notification)
	if err != nil {
		return []byte{}, err
	}

	res, err := h.client.Do(req)
	if err != nil {
		return []byte{}, err
	}

	defer func() {
		_ = res.Body.Close()
	}()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return []byte{}, err
	}

	if statusOK := res.StatusCode >= 200 && res.StatusCode < 300; !statusOK { // nolint: gomnd
		return body, ErrInvalidHTTPStatusCode
	}

	return body, nil
}

// NewHTTPPostWriter is the constructor of NewHTTPPostWriter.
func NewHTTPPostWriter[T Writable](url string, timeout time.Duration) *HTTPPostWriter[T] {
	return &HTTPPostWriter[T]{
		url: url,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}
