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

var serv http.Server
var fnvHash hash.Hash64

func hashHandler(w http.ResponseWriter, req *http.Request) {
    if req.URL.Path != "/hash" {

        http.Error(w, "404 not found.", http.StatusNotFound)
        return
    }


    if req.Method == "GET" {
        // log.Println(req.Body)
        // Sleep for 250 ms
        time.Sleep(250 * time.Millisecond)
        // First read the resquest's body into a byte slice
        reqBody, err := ioutil.ReadAll(req.Body)
        if err != nil {
            log.Fatal(err)
        }
        // log.Println(reqBody)
        // log.Println(string(reqBody))

        // Next unmarshal the byte slice into the buffer
        buffer := make([]byte, int(req.ContentLength))
        json.Unmarshal(reqBody, &buffer)

        // log.Printf("Result unmarshal: %v\n", buffer)

        // Get hash value of packet
        hashValue := get64FNV1aHash(buffer)

        // Clear the buffer's contents
        buffer = nil
        buffer = make([]byte, 8)

        // Convert the hashValue into a byte slice
        binary.BigEndian.PutUint64(buffer, hashValue)
        // log.Printf("Buffer with hash: %v\n", buffer)
        // log.Printf("Length of buffer with hash: %d\n", len(buffer))

        // Finally write the encoded buffer with the hash back to the recipient
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(buffer)
        return
    } else {
        http.Error(w, "Method is not supported.", http.StatusNotFound)
        return
    }
}

func get64FNV1aHash(packet []byte) (uint64){
    defer fnvHash.Reset()
    fnvHash.Write(packet)
    hashValue := fnvHash.Sum64()
    // log.Printf("Packet to hash: %v\n", packet)
    // log.Printf("Hash value: %x\n", hashValue)

    return hashValue
}

func main() {
    // Command line args
    var backendPortNum = flag.String("backend_port", "8080", "Port number of the HTTP backend server (i.e. 80)")
    // var backendHashEndpoint = flag.String("backend_hash", "/hash", "The hash endpoint of the HTTP backend server (i.e. hash)")
    // var backendShutdownEndpoint = flag.String("backend_shutdown", "/shutdown", "The hash endpoint of the HTTP backend server (i.e. shutdown)")
    var wTimeLimit = flag.Int("w_time", 5, "Max number of seconds the HTTP backend server entire will spend reading the request, including the body (i.e. 5)")
    var rTimeLimit = flag.Int("r_time", 5, "Max number of seconds the HTTP backend server will wait before timing out writes of the response (i.e. 5)")
    flag.Parse()

    // Initialize hashing object
    fnvHash = fnv.New64a()

    // Create new HTTP request multiplexer for server
    m := http.NewServeMux()
    // Initialize the HTTP server
    service := ":" + *backendPortNum
    serv = http.Server {Addr: service,
                        Handler: m,
                        ReadTimeout: time.Duration(*wTimeLimit) * time.Second,
                        WriteTimeout: time.Duration(*rTimeLimit) * time.Second,
    }

    // Add specific context to allow for graceful shutdown
    // Using the context's cancel function prevents shutdown from being called multiple times
    // SOURCE: https://medium.com/@int128/shutdown-http-server-by-endpoint-in-go-2a0e2d7f9b8c
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

    log.Printf("Started server at port %v (change to 80 later bc `listen tcp :80: bind: permission denied`)\n", serv.Addr)

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
    log.Printf("Finished")
}
