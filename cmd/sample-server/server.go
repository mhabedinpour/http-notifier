package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const DefaultPort = 8090
const ServerTimeout = 5 * time.Second

func main() {
	port := flag.Int("port", DefaultPort, "server port to listen")
	flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.Body == nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("request body cannot be empty"))

			return
		}

		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			log.Println("could not read request body", err)

			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("could not read request body"))

			return
		}
		defer func() {
			_ = req.Body.Close()
		}()

		log.Println("request body", string(bodyBytes))

		w.WriteHeader(http.StatusNoContent)
		_, _ = w.Write([]byte{})
	})

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", *port),
		ReadHeaderTimeout: ServerTimeout,
	}
	err := server.ListenAndServe()
	if err != nil {
		log.Fatal("could not listen", err)
	}
}
