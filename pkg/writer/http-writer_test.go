package writer

import (
	"bytes"
	"errors"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPPostWriter_Write(t *testing.T) {
	c := 0
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if c == 0 {
			_, _ = rw.Write([]byte("OK"))
			c++
		} else {
			rw.WriteHeader(http.StatusBadRequest)
			_, _ = rw.Write([]byte("NOT OK"))
		}
	}))

	invalidWriter := NewHTTPPostWriter[*bytes.Buffer]("test", 5*time.Second)
	res, err := invalidWriter.Write(bytes.NewBufferString("hello"))
	assert.NotNil(t, err)

	writer := NewHTTPPostWriter[*bytes.Buffer](server.URL, 5*time.Second)

	res, err = writer.Write(bytes.NewBufferString("hello"))
	assert.Nil(t, err)
	assert.Equal(t, res, []byte("OK"))

	res, err = writer.Write(bytes.NewBufferString("hello"))
	assert.Equal(t, errors.Is(err, ErrInvalidHTTPStatusCode), true)
	assert.Equal(t, res, []byte("NOT OK"))
}
