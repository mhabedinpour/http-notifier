package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/mhabedinpour/http-notifier/pkg/notifier"

	"github.com/muesli/cancelreader"
	"github.com/sony/gobreaker"
)

const (
	DefaultInterval = 1 * time.Second

	HTTPTimeout = 10 * time.Second

	MaxInFlightRequests    = 50
	MaxQueuedNotifications = notifier.QueueNoLimit
	RetryWait              = 1 * time.Second
	MaxAttempts            = 5

	StdinBufferSize = 100
)

func retryHandler(_ notifier.Notification, attempts int, _ error) time.Duration {
	if attempts >= MaxAttempts {
		return notifier.NoRetry
	}

	return RetryWait // we can also use an exponential backoff instead of this constant
}

func newCB() notifier.CircuitBreaker {
	var st gobreaker.Settings
	st.Name = "Notifier"

	cb := gobreaker.NewCircuitBreaker(st)

	return func(req func() error) error {
		_, err := cb.Execute(func() (interface{}, error) {
			return nil, req()
		})

		return err
	}
}

// readStdin reads from stdin line by line using a scanner and sends each line to a channel.
func readStdin(stdin io.Reader, ctxCancel context.CancelFunc) <-chan string {
	lines := make(chan string, StdinBufferSize)

	go func() {
		defer close(lines)
		defer ctxCancel() // when stdin is piped, we need to cancel the ctx after stdin is closed in order to exit automatically.

		scan := bufio.NewScanner(stdin)
		for scan.Scan() {
			s := scan.Text()
			lines <- s
		}
	}()

	return lines
}

// enqueueStdin writes the stdin channel to the queue.
func enqueueStdin(stdinChannel <-chan string, queue *notifier.Queue[notifier.Notification]) {
	for {
		select {
		case notification, ok := <-stdinChannel:
			if !ok {
				return
			}

			_ = queue.Enqueue(bytes.NewBufferString(notification))
		default:
			return
		}
	}
}

func getStdin() (io.Reader, func()) {
	fi, err := os.Stdin.Stat()
	if err != nil {
		log.Fatalln("could not get stdin stats", err)
	}

	if (fi.Mode() & os.ModeCharDevice) == 0 {
		// if stdin is piped, return the original stdin
		return os.Stdin, func() {
			_ = os.Stdin.Close()
		}
	}

	// if not piped, create a cancellable reader. for graceful shutdown we need to stop the readStdin goroutine, cancelReader can help by interrupting the blocked read syscall (if any) in readStdin scanner.
	stdinCancellable, err := cancelreader.NewReader(os.Stdin)
	if err != nil {
		log.Fatalln("could not create cancellable stdin", err)
	}

	return stdinCancellable, func() {
		_ = stdinCancellable.Cancel()
	}
}

func main() {
	url := flag.String("url", "http://127.0.0.1:8090", "URL to send HTTP POST requests to")
	interval := flag.Duration("interval", DefaultInterval, "Interval for reading notifications from stdin and sending them")
	flag.Parse()

	writer := notifier.NewHTTPPostWriter[notifier.Notification](*url, HTTPTimeout)
	queue, err := notifier.NewQueue[notifier.Notification](writer, notifier.QueueOptions[notifier.Notification]{
		MaxInFlightRequests:    MaxInFlightRequests,
		MaxQueuedNotifications: MaxQueuedNotifications,
		RetryHandler:           retryHandler,
		CircuitBreaker:         newCB(),
	})
	if err != nil {
		log.Fatalln("could not create queue", err)
	}

	stdin, closeStdin := getStdin()
	ctx, ctxCancel := context.WithCancel(context.Background())
	stdinChannel := readStdin(stdin, ctxCancel)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			select {
			case <-queue.Successes():
				// ignore successes, we should read from this channel to avoid blocking in the queue workers.
			case err, ok := <-queue.Errors():
				if !ok {
					return
				}

				log.Println("could not send notification", err.Notification, err.Attempts, err.Err)
			}
		}
	}()

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	ticker := time.NewTicker(*interval)
	defer stop()

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()

			closeStdin() // stop readStdin goroutine by closing its file.
			// read anything left in the stdin for one last time. enqueueStdin cannot be used here because readStdin goroutine may get blocked when writing to the stdin channel.
			// if golang runtime schedules enqueueStdin goroutine sooner than the readStdin goroutine, the default case in enqueueStdin will stop the goroutine and there may be some messages left in the stdin.
			for notification := range stdinChannel {
				_ = queue.Enqueue(bytes.NewBufferString(notification))
			}

			queue.Stop()

			wg.Wait() // make sure all successes and errors are handled

			return
		case <-ticker.C:
			enqueueStdin(stdinChannel, queue)
		}
	}
}
