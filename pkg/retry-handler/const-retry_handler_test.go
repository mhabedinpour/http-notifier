package retryhandler

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestConstRetryHandler_CalculateSleep(t *testing.T) {
	handler := NewConstRetryHandler[any](100*time.Millisecond, 5)

	assert.Equal(t, handler.CalculateSleep(nil, 1, nil), 100*time.Millisecond)
	assert.Equal(t, handler.CalculateSleep(nil, 2, nil), 100*time.Millisecond)
	assert.Equal(t, handler.CalculateSleep(nil, 5, nil), NoRetry)
	assert.Equal(t, handler.CalculateSleep(nil, 6, nil), NoRetry)
}
