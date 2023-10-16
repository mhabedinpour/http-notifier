package circuitbreaker

import "github.com/sony/gobreaker"

// Gobreaker is an implementation of CircuitBreaker which uses gobreaker internally.
type Gobreaker[R any] struct {
	gobreaker *gobreaker.CircuitBreaker
}

// to make sure Gobreaker implements CircuitBreaker.
var _ CircuitBreaker[any] = (*Gobreaker[any])(nil)

func (g Gobreaker[R]) Execute(req func() (R, error)) (R, error) {
	result, err := g.gobreaker.Execute(func() (any, error) {
		return req()
	})
	if err != nil {
		var r R

		return r, err
	}

	assertedResult, ok := result.(R)
	if !ok {
		panic("invalid data type in breaker")
	}

	return assertedResult, nil
}

// NewGobreaker is the constructor of Gobreaker.
func NewGobreaker[R any](settings gobreaker.Settings) Gobreaker[R] {
	return Gobreaker[R]{
		gobreaker: gobreaker.NewCircuitBreaker(settings),
	}
}
