package main

import (
	"log"
	"net/http"
	"context"
	"encoding/json"
	"io/ioutil"
	"hash"
	"hash/fnv"
	"encoding/binary"
	"time"
	"flag"
)

// Global variable for the hashing object
var fnvHash hash.Hash64

// Handler for any requests with the /hash endpoint
func hashHandler(w http.ResponseWriter, req *http.Request) {
	// Check if this handler got the correct endpoint
	if req.URL.Path != "/hash" {
		http.Error(w, "404 not found.", http.StatusNotFound)
		return
	}

	// Only satisfy GET requests
	if req.Method == "GET" {
		// First read the resquest's body into a byte slice
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Fatal(err)
		}

		// Next unmarshal the byte slice into the buffer
		buffer := make([]byte, int(req.ContentLength))
		json.Unmarshal(reqBody, &buffer)

		// Sleep for 250 ms
		time.Sleep(250 * time.Millisecond)

		// Get hash value of packet
		hashValue := get64FNV1aHash(buffer)

		// Clear the buffer's contents
		buffer = nil
		buffer = make([]byte, 8)

		// Convert the hashValue into a byte slice
		binary.BigEndian.PutUint64(buffer, hashValue)

		// Finally write the encoded buffer with the hash back to the recipient
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(buffer)
		return
	} else {
		// Handle all unsuported request types
		http.Error(w, "Method is not supported.", http.StatusNotFound)
		return
	}
}

// Calculates the 64bit fnv1a hash of a given byte slice
func get64FNV1aHash(packet []byte) (uint64){
	defer fnvHash.Reset()
	fnvHash.Write(packet)
	hashValue := fnvHash.Sum64()

	return hashValue
}

// Create the HTTP server and listen and serve incoming requests
func main() {
	// Command line args
	var backendPortNum = flag.String("port", "80", "Port number of the HTTP backend server (i.e. 80)")
	var rhTimeLimit = flag.Int("rh_time", 20, "Max number of seconds the HTTP backend server entire will spend reading the headers of the request (i.e. 20)")
	var wTimeLimit = flag.Int("w_time", 20, "Max number of seconds the HTTP backend server will wait before timing out writes of the response (i.e. 20)")
	flag.Parse()

	// Initialize hashing object
	fnvHash = fnv.New64a()

	// Create new HTTP request multiplexer for server
	m := http.NewServeMux()
	// Create the HTTP server
	service := ":" + *backendPortNum
	serv := http.Server {   Addr: service,
							Handler: m,
							ReadHeaderTimeout: time.Duration(*rhTimeLimit) * time.Second,
							WriteTimeout: time.Duration(*wTimeLimit) * time.Second,
	}

	// Add specific context to allow for graceful shutdown
	// Using the context's cancel function prevents shutdown from being called multiple times
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Add the handler for the shutdown endpoint
	m.HandleFunc("/shutdown", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/shutdown" {
			http.Error(w, "404 not found.", http.StatusNotFound)
			return
		}

		w.Write([]byte("Shutdown HTTP server"))
		// Cancel the context on request
		cancel()
	})

	// Add the handler for the hash endpoint
	m.HandleFunc("/hash", hashHandler)

	log.Printf("Started HTTP server at %v\n", serv.Addr)

	// Listen and serve through a goroutine to allow for graceful shutdown
	go func() {
		err := serv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	select {
	case <-ctx.Done():
		// Shutdown the server when the context is canceled
		serv.Shutdown(ctx)
	}

	log.Printf("HTTP server has been shutdown")
}
