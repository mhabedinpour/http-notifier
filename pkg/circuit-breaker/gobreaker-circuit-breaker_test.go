package circuitbreaker

import (
	"errors"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGobreaker_Execute(t *testing.T) {
	var st gobreaker.Settings
	st.Name = "Notifier"
	cb := NewGobreaker[bool](st)

	res, err := cb.Execute(func() (bool, error) {
		return true, nil
	})
	assert.Equal(t, res, true)
	assert.Nil(t, err)

	sampleErr := errors.New("error")
	res, err = cb.Execute(func() (bool, error) {
		return false, sampleErr
	})
	assert.Equal(t, res, false)
	assert.Equal(t, err, sampleErr)
}
