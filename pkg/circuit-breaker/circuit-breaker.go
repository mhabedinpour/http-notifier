package circuitbreaker

// CircuitBreaker is used to implement Circuit Breaker pattern and avoid cascading failures when the down-stream systems are down.
type CircuitBreaker[R any] interface {
	Execute(req func() (R, error)) (R, error)
}
