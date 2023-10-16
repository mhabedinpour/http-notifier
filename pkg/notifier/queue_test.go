package notifier

import (
	"bytes"
	"errors"
	circuitbreaker "github.com/mhabedinpour/http-notifier/pkg/circuit-breaker"
	retryhandler "github.com/mhabedinpour/http-notifier/pkg/retry-handler"
	"sync"
	"testing"
	"time"

	"github.com/mhabedinpour/http-notifier/pkg/writer"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewQueue(t *testing.T) {
	t.Run("InvalidMaxInFlightRequests", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockWriter := writer.NewMockWriter[Notification, any](ctrl)
		mockCb := circuitbreaker.NewMockCircuitBreaker[any](ctrl)

		_, err := NewQueue[Notification, any](mockWriter, nil, mockCb, QueueOptions[Notification]{
			MaxInFlightRequests:    0,
			MaxQueuedNotifications: QueueNoLimit,
		})

		assert.NotNil(t, err)
	})

	t.Run("InvalidMaxQueuedNotifications", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockWriter := writer.NewMockWriter[Notification, any](ctrl)
		mockCb := circuitbreaker.NewMockCircuitBreaker[any](ctrl)

		_, err := NewQueue[Notification, any](mockWriter, nil, mockCb, QueueOptions[Notification]{
			MaxInFlightRequests:    1,
			MaxQueuedNotifications: 0,
		})

		assert.NotNil(t, err)
	})

	t.Run("NilCircuitBreaker", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockWriter := writer.NewMockWriter[Notification, any](ctrl)

		_, err := NewQueue[Notification, any](mockWriter, nil, nil, QueueOptions[Notification]{
			MaxInFlightRequests:    1,
			MaxQueuedNotifications: 1,
		})

		assert.NotNil(t, err)
	})

	t.Run("Valid", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockWriter := writer.NewMockWriter[Notification, any](ctrl)
		mockCb := circuitbreaker.NewMockCircuitBreaker[any](ctrl)

		queue, err := NewQueue[Notification, any](mockWriter, nil, mockCb, QueueOptions[Notification]{
			MaxInFlightRequests:    1,
			MaxQueuedNotifications: QueueNoLimit,
		})
		defer queue.Stop()

		assert.Nil(t, err)
	})
}

func TestQueue_Enqueue(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockWriter := writer.NewMockWriter[Notification, any](ctrl)
		mockCb := circuitbreaker.NewMockCircuitBreaker[any](ctrl)

		queue, err := NewQueue[Notification, any](mockWriter, nil, mockCb, QueueOptions[Notification]{
			MaxInFlightRequests:    1,
			MaxQueuedNotifications: QueueNoLimit,
		})
		defer queue.Stop()
		assert.Nil(t, err)

		notif1 := bytes.NewBufferString("notif1")
		res1 := bytes.NewBufferString("res1")
		mockWriter.EXPECT().Write(notif1).Times(1).Return(res1, nil)
		mockCb.EXPECT().Execute(gomock.Any()).Times(1).DoAndReturn(
			func(req func() (any, error)) (any, error) {
				return req()
			},
		)
		err = queue.Enqueue(notif1)
		assert.Nil(t, err)

		success := <-queue.Successes()
		assert.Equal(t, success.Notification, notif1)
		assert.Equal(t, success.Result, res1)
	})

	t.Run("Failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockWriter := writer.NewMockWriter[Notification, any](ctrl)
		mockCb := circuitbreaker.NewMockCircuitBreaker[any](ctrl)

		queue, err := NewQueue[Notification, any](mockWriter, nil, mockCb, QueueOptions[Notification]{
			MaxInFlightRequests:    1,
			MaxQueuedNotifications: QueueNoLimit,
		})
		defer queue.Stop()
		assert.Nil(t, err)

		notif1 := bytes.NewBufferString("notif1")
		res1 := bytes.NewBufferString("res1")
		err1 := errors.New("error1")
		mockWriter.EXPECT().Write(notif1).Times(1).Return(res1, err1)
		mockCb.EXPECT().Execute(gomock.Any()).Times(1).DoAndReturn(
			func(req func() (any, error)) (any, error) {
				return req()
			},
		)
		err = queue.Enqueue(notif1)
		assert.Nil(t, err)

		queueErr := <-queue.Errors()
		assert.Equal(t, queueErr.Notification, notif1)
		assert.Equal(t, queueErr.Err, err1)
		assert.Equal(t, queueErr.Attempts, 1)
	})

	t.Run("Retry", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockWriter := writer.NewMockWriter[Notification, any](ctrl)
		mockCb := circuitbreaker.NewMockCircuitBreaker[any](ctrl)
		mockRh := retryhandler.NewMockRetryHandler[Notification](ctrl)

		queue, err := NewQueue[Notification, any](mockWriter, mockRh, mockCb, QueueOptions[Notification]{
			MaxInFlightRequests:    1,
			MaxQueuedNotifications: QueueNoLimit,
		})
		defer queue.Stop()
		assert.Nil(t, err)

		notif1 := bytes.NewBufferString("notif1")
		res1 := bytes.NewBufferString("res1")
		err1 := errors.New("error1")
		mockRh.EXPECT().CalculateSleep(notif1, 1, err1).Times(1).Return(100 * time.Millisecond)
		mockRh.EXPECT().CalculateSleep(notif1, 2, err1).Times(1).Return(100 * time.Millisecond)
		mockWriter.EXPECT().Write(notif1).Times(2).Return(res1, err1)
		mockWriter.EXPECT().Write(notif1).Times(1).Return(res1, nil)
		mockCb.EXPECT().Execute(gomock.Any()).Times(3).DoAndReturn(
			func(req func() (any, error)) (any, error) {
				return req()
			},
		)
		err = queue.Enqueue(notif1)
		assert.Nil(t, err)

		success := <-queue.Successes()
		assert.Equal(t, success.Notification, notif1)
		assert.Equal(t, success.Result, res1)
	})

	t.Run("RetrySuccess", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockWriter := writer.NewMockWriter[Notification, any](ctrl)
		mockCb := circuitbreaker.NewMockCircuitBreaker[any](ctrl)

		queue, err := NewQueue[Notification, any](mockWriter, retryhandler.NewConstRetryHandler[Notification](100*time.Millisecond, 2), mockCb, QueueOptions[Notification]{
			MaxInFlightRequests:    1,
			MaxQueuedNotifications: QueueNoLimit,
		})
		defer queue.Stop()
		assert.Nil(t, err)

		notif1 := bytes.NewBufferString("notif1")
		res1 := bytes.NewBufferString("res1")
		err1 := errors.New("error1")
		mockWriter.EXPECT().Write(notif1).Times(1).Return(res1, err1)
		mockWriter.EXPECT().Write(notif1).Times(1).Return(res1, nil)
		mockCb.EXPECT().Execute(gomock.Any()).Times(2).DoAndReturn(
			func(req func() (any, error)) (any, error) {
				return req()
			},
		)
		err = queue.Enqueue(notif1)
		assert.Nil(t, err)

		success := <-queue.Successes()
		assert.Equal(t, success.Notification, notif1)
		assert.Equal(t, success.Result, res1)
	})

	t.Run("RetryFailure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockWriter := writer.NewMockWriter[Notification, any](ctrl)
		mockCb := circuitbreaker.NewMockCircuitBreaker[any](ctrl)

		queue, err := NewQueue[Notification, any](mockWriter, retryhandler.NewConstRetryHandler[Notification](100*time.Millisecond, 3), mockCb, QueueOptions[Notification]{
			MaxInFlightRequests:    1,
			MaxQueuedNotifications: QueueNoLimit,
		})
		defer queue.Stop()
		assert.Nil(t, err)

		notif1 := bytes.NewBufferString("notif1")
		res1 := bytes.NewBufferString("res1")
		err1 := errors.New("error1")
		mockWriter.EXPECT().Write(notif1).Times(3).Return(res1, err1)
		mockCb.EXPECT().Execute(gomock.Any()).Times(3).DoAndReturn(
			func(req func() (any, error)) (any, error) {
				return req()
			},
		)
		err = queue.Enqueue(notif1)
		assert.Nil(t, err)

		queueErr := <-queue.Errors()
		assert.Equal(t, queueErr.Notification, notif1)
		assert.Equal(t, queueErr.Err, err1)
		assert.Equal(t, queueErr.Attempts, 3)
	})

	t.Run("RetryAfterStop", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockWriter := writer.NewMockWriter[Notification, any](ctrl)
		mockCb := circuitbreaker.NewMockCircuitBreaker[any](ctrl)

		queue, err := NewQueue[Notification, any](mockWriter, retryhandler.NewConstRetryHandler[Notification](100*time.Millisecond, 3), mockCb, QueueOptions[Notification]{
			MaxInFlightRequests:    1,
			MaxQueuedNotifications: QueueNoLimit,
		})
		assert.Nil(t, err)

		notif1 := bytes.NewBufferString("notif1")
		res1 := bytes.NewBufferString("res1")
		err1 := errors.New("error1")
		mockWriter.EXPECT().Write(notif1).Times(1).Return(res1, err1)
		mockWriter.EXPECT().Write(notif1).MaxTimes(1).Return(res1, nil)
		mockCb.EXPECT().Execute(gomock.Any()).MaxTimes(3).DoAndReturn(
			func(req func() (any, error)) (any, error) {
				return req()
			},
		)
		err = queue.Enqueue(notif1)
		assert.Nil(t, err)

		queue.Stop()

		queueErr := <-queue.Errors()
		assert.Equal(t, queueErr.Notification, notif1)
		assert.Equal(t, queueErr.Err, err1)
		assert.Equal(t, queueErr.Attempts, 1)
	})
}

func TestQueue_Stop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockWriter := writer.NewMockWriter[Notification, any](ctrl)
	mockCb := circuitbreaker.NewMockCircuitBreaker[any](ctrl)

	queue, err := NewQueue[Notification, any](mockWriter, nil, mockCb, QueueOptions[Notification]{
		MaxInFlightRequests:    1,
		MaxQueuedNotifications: QueueNoLimit,
	})
	assert.Nil(t, err)

	notif1 := bytes.NewBufferString("notif1")
	res1 := bytes.NewBufferString("res1")
	notif2 := bytes.NewBufferString("notif2")
	res2 := bytes.NewBufferString("res2")
	mockWriter.EXPECT().Write(notif1).Times(1).Return(res1, nil)
	mockWriter.EXPECT().Write(notif2).Times(1).Return(res2, nil)
	mockCb.EXPECT().Execute(gomock.Any()).Times(2).DoAndReturn(
		func(req func() (any, error)) (any, error) {
			return req()
		},
	)

	err = queue.Enqueue(notif1)
	assert.Nil(t, err)
	err = queue.Enqueue(notif2)
	assert.Nil(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		success1 := <-queue.Successes()
		assert.Equal(t, success1.Notification, notif1)
		assert.Equal(t, success1.Result, res1)
		success2 := <-queue.Successes()
		assert.Equal(t, success2.Notification, notif2)
		assert.Equal(t, success2.Result, res2)

		wg.Done()
	}()

	queue.Stop()

	wg.Wait()

	err = queue.Enqueue(notif2)
	assert.Equal(t, errors.Is(err, ErrQueueStopped), true)
}
