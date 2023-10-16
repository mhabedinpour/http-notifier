# HTTP Notifier

## Packages:

1. `pkg/circuit-breaker`: Contains an interface of a generic circuit breaker and an implementation of it based on [`gobreaker`](https://github.com/sony/gobreaker).
2. `pkg/retry-handler`: Contains an interface for a generic retry handler and a constant retry handler which waits for a constant time between each attempt until maximum attempts are reached.
3. `pkg/writer`: Contains an interface of a generic external entity for writing notifications. Also, an HTTP POST writer is implemented.
4. `pkg/notifier`: Contains a queue which writes notifications to a `Writer` asynchronously with the ability to control the maximum number of concurrent writes and the maximum number of queued items. If queue becomes full, Older messages will be discarded. Clients can use `Successes` and `Errors` channels to get updates about the enqueued notifications. Also retry pattern and circuit breaker patterns are used in the queue.

## Commands:

1. `make generate`: Generates mock implementations for interfaces automatically.
2. `make lint`: Runs `golangci-lint` on the project.
3. `make test`: Runs automated tests.

## Executables:

1. `cmd/sample-server/server.go`: A sample HTTP server which logs the body of all incoming requests and returns `http.StatusNoContent` to the client.
2. `cmd/queue-client/client.go`: A sample client for `pkg/notifier/queue`. Reads messages from stdin in an interval and posts them to an HTTP server. Also, graceful shutdown is implemented to drain the queue exiting. 
