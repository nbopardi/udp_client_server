package main

import (
	"log"
	"net"
	"net/http"
	"encoding/json"
    "io/ioutil"
	"bytes"
	"time"
    "runtime"
	"sync"
	"strconv"
	"flag"
)

// Packet struct that is used for reflecting a packet back to its sender
// Packet: a byte slice representing the packet's payload
// Addr: a UDP address from the sender of the packet
type PacketStruct struct {
	Packet 	[]byte
	Addr 	*net.UDPAddr
}

// Reflect packets from a channel back to the client
func reflectPacket(conn *net.UDPConn, writeTimeLimit time.Duration, packetsSentCounter *int, writeOut <-chan PacketStruct, wg *sync.WaitGroup) {
	// Close wait group when done
	defer wg.Done()

    // Execute this goroutine on its own exclusive OS thread
    runtime.LockOSThread()

    // Loop for sending packets back to the client
    // Exited when the write channel is closed and drained
	reflectLoop:
		for {
			select {
			case packet, ok := <- writeOut:
				// If the channel has been closed and drained, exit the outer loop
				if !ok {
					break reflectLoop
				} else {
					// Set a deadline for how long server should wait to write message
					err := conn.SetWriteDeadline(time.Now().Add(writeTimeLimit))
					if err != nil {
						log.Println("Could not set write deadline for connection: ", err)
					}

					// Reflect the message back to the client
					_, err = conn.WriteToUDP(packet.Packet, packet.Addr)
					// Error handling
					if err != nil {
						log.Println("Could not write message to UDP client: ", err)
					} else {
						// Increment the counter for the number of packets sent back
						*packetsSentCounter++
					}
				}
			default:
				// Do nothing
			}
		}

    // Unlock the OS thread for other goroutines to use
    runtime.UnlockOSThread()
}

// Communicates with the HTTP backend server
// Gets the hash of a packet's payload and appends it to the payload before inserting it into the write channel
func commBackend(client *http.Client, hashURL string, packet PacketStruct, writeOut chan <- PacketStruct, tokens <-chan struct{}, wgBackend *sync.WaitGroup) {
    // Close wait group when done
    defer wgBackend.Done()

	// Marshal the packet's payload
	requestBody, err := json.Marshal(packet.Packet)
    if err != nil {
        log.Printf("Could not marshal the packet payload: %v\n", err)
        return
    }
    // Create a new HTTP GET Request for the /hash endpoint
    request, err := http.NewRequest("GET", hashURL, bytes.NewBuffer(requestBody))
    request.Header.Set("Content-type", "application/json")
    if err != nil {
        log.Printf("Could not create HTTP GET request: %v\n", err)
        return
    }

    // Send the request and acquire a response
    resp, err := client.Do(request)
    if err != nil {
    	// log.Printf("Could not send and acquire a response from the HTTP backend: %v\n", err)
        return
    }

    // Read through the body of the response
    body, err := ioutil.ReadAll(resp.Body)
    // Close the body of the response
    resp.Body.Close()
    if err != nil {
        log.Printf("Could not read the response body: %v\n", err)
        return
    }

    // Unmarshal the hash into a byte slice
    buffer := make([]byte, 8)
    err = json.Unmarshal(body, &buffer)
    if err != nil {
        log.Printf("Could not unmarshal the hash into a byte slice: %v\n", err)
        return
    }

    // Append the hash to the end of the packet's payload
    packet.Packet = append(packet.Packet[:], buffer[:]...)

    // Write the packet to the out channel to be reflected back to the client
    writeOut <- packet

    // Release a token, indicating that this job is finished
    <- tokens
}

// Handles the spawning of goroutines for backend communication
// Process stops once the UDP server stops receiving from the UDP client and shuts down the HTTP backend server
func hashPacket(client *http.Client, hashURL string, shutdownURL string, pool *sync.Pool, doneChan <-chan struct{}, writeOut chan<- PacketStruct, numConcurrentJobs int, wg *sync.WaitGroup) {
	// Close wait group when done
	defer wg.Done()

    // Execute this goroutine on its own exclusive OS thread
    runtime.LockOSThread()

	// Create a separate wait group for all requests made to the backend
	var wgBackend sync.WaitGroup

	// Create a channel to limit the number of concurrent goroutines
	// This acts like a counting semaphore / rate limiter
	// numConcurrentJobs must be less than ulimit -n (max number of open file descriptors)
	var tokens = make(chan struct{}, numConcurrentJobs)

	// Extract packets from a pool and spawn a goroutine to get the fnv1a hash of the packet and
    // append it to the packet's payload before inserting it into the write channel
	hashLoop:
		for {
			select {
            case <-doneChan:
                // We are done receiving messages from client, so no longer need to reflect any back
                log.Println("Stopped receiving, so no need to communicate with backend.")
                break hashLoop
            default:
                // Get a packet from the pool
                packet := pool.Get().(*PacketStruct)

                // Verify that the packet has contents and is not an empty struct
                // by checking its address field
                if ((*packet).Addr.String() != "<nil>") {
                    // Acquire a token for communicating with HTTP backend
                    // If the max number of goroutines (numConcurrentJobs) for communicating with the backend
                    // has been reached, this action blocks until one of those goroutines has finished
                    tokens <- struct{}{}

                    // Add a process to the wait group for backend communication
                    wgBackend.Add(1)

                    // Communicate with the HTTP backend server
                    go commBackend(client, hashURL, *packet, writeOut, tokens, &wgBackend)
                }
            }
        }

	// Wait for all requests to the backend to finish
	wgBackend.Wait()
    log.Println("All remaining goroutine communication with backend are complete")

	// Close the channel when done hashing the packets
	close(writeOut)

	// Create a request to shutdown the HTTP backend server
	request, err := http.NewRequest("GET", shutdownURL, nil)
	if err != nil {
        log.Println(err)
    }
    // Send the request and output the response
    resp, err := client.Do(request)
    if err != nil {
        log.Println(err)
    }
    body, err := ioutil.ReadAll(resp.Body)
    resp.Body.Close()
    if err != nil {
        log.Printf("Could not shutdown HTTP backend: %v\n", err)
    } else {
        log.Println(string(body))
    }

    // Close all idle connections for the HTTP backend client
    client.CloseIdleConnections()

    // Unlock the OS thread for other goroutines to use
    runtime.UnlockOSThread()
}

// Receives a packet on the UDP connection until no longer receiving a response from a client
// Inputs all packets received to a pool to be accessed for communicating with the HTTP backend
func recvPacket(conn *net.UDPConn, readTimeLimit time.Duration, packetsRecvCounter *int, pool *sync.Pool, doneChan chan<- struct{}, wg *sync.WaitGroup) {
	// Close wait group when done
	defer wg.Done()

    // Execute this goroutine on its own exclusive OS thread
    runtime.LockOSThread()

	// Loop to handle reading packets from client
	// Exited when time limit for waiting on client request is reached
	receiveSendLoop:
		for {
			// Create buffer to read in message
			buffer := make([]byte, 100)

			// Set time limit for how long to wait for client response
			err := conn.SetReadDeadline(time.Now().Add(readTimeLimit))
			if err != nil {
				log.Println("Could not set read deadline for connection: ", err)
			}

			// Read message from client
			n, addr, err := conn.ReadFromUDP(buffer)

			// Exit from loop if read time limit reached
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
						log.Println("Time limit reached for awaiting client request. No longer receiving.")
						break receiveSendLoop
				}
				log.Fatal("Could not receive message from UDP client: ", err)
			} else {
                // Place the packet in a pool
                pool.Put(&PacketStruct{buffer[:n], addr})

				// Increment the counter for number of packets received
				*packetsRecvCounter++
			}

		}

    // Insert an empty struct into the channel to signify that done reading messages from UDP client
    doneChan <- struct{}{}

    // Unlock the OS thread for other goroutines to use
    runtime.UnlockOSThread()
}

// Main function to set up a UDP server that listens for packets sent from a UDP client
// The server makes a call to the HTTP backend server to get the fnv1a hash of each packet
// The hash is appended to the end of each packet's payload and reflected back to the UDP client
func main() {
	// Command line args
	var backendHostName = flag.String("backend_host", "localhost", "IPv4 of the HTTP backend server (i.e. 169.254.105.13)")
	var backendPortNum = flag.String("backend_port", "80", "Port number of the HTTP backend server (i.e. 80)")
	var portNum = flag.String("port", "40000", "Port number of the server (i.e. 40000)")
	var wTimeLimit = flag.Int("w_time", 5, "Max number of seconds the connection will wait on a full send queue to free up to send a packet (i.e. 5)")
	var rTimeLimit = flag.Int("r_time", 10, "Max number of seconds the server will wait to receive a request from client before closing (i.e. 10)")
	var numConcurrentJobs = flag.Int("n_jobs", 10000, "Max number of requests made to the HTTP backend occurring at the same time (i.e. 10000)")
	var expectContTime = flag.Int("ec_time", 4, "Amount of seconds to wait for the HTTP backend's first response headers after fully writing the request headers (i.e. 4)")
    var respHeaderTime = flag.Int("rh_time", 10, "Amount of seconds to wait for the HTTP backend's response headers after fully writing the request and body (i.e. 10)")
    var idleConnTime = flag.Int("ic_time", 10, "Max amount of seconds an idle (keep-alive) connection will remain idle before closing itself (i.e. 10)")
    var idleConnsPerHost = flag.Int("iconn_host", 10000, "Max idle (keep-alive) connections to keep per-host (i.e. 10000)")
    var chanCap = flag.Int("buffer", 500000, "Max buffer size of the channel used to store received packets that are reflected to client (i.e. 500000)")
	flag.Parse()

	// Define the HTTP backend server address
	backendService := "http://" + *backendHostName + ":" + *backendPortNum
	hashURL := backendService + "/hash"
	shutdownURL := backendService + "/shutdown"

	// Create a transport for the HTTP client
	tr := &http.Transport {     ExpectContinueTimeout: time.Duration(*expectContTime) * time.Second,
                                ResponseHeaderTimeout: time.Duration(*respHeaderTime) * time.Second,
                                IdleConnTimeout: time.Duration(*idleConnTime) * time.Second,
                                DisableKeepAlives: false,
                                MaxIdleConnsPerHost: *idleConnsPerHost,
                                MaxIdleConns: 0,
                                MaxConnsPerHost: 0,
                                WriteBufferSize: 0,
                                ReadBufferSize: 0,
    }

	// Create a client with a specific transport
	backendClient := &http.Client{Transport: tr}

	// Define the server address
	// No host provided so that ResolveUDPAddr resolves to the addreess of UDP endpoint
	service := ":" + *portNum
	networkName := "udp4"

	// Get address of UDP endpoint
	udpAddr, err := net.ResolveUDPAddr(networkName, service)
	if err != nil {
		log.Fatal(err)
	}

	// Setup listener for incoming UDP connection
	udpConn, err := net.ListenUDP(networkName, udpAddr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("UDP server up and listening on port %s \n", *portNum)

	// Close the UDP connection when done with everything
	defer udpConn.Close()

	// Set a read deadline for how long should wait for client response
	readTimeLimit := time.Duration(*rTimeLimit) * time.Second
	// Set a write deadline for how long should wait on a full send queue to free up to send a packet
	writeTimeLimit := time.Duration(*wTimeLimit) * time.Second

	// Create channel to signal when done reading packets from client
    doneChan := make(chan struct{}, 1)
	// Create channel to hold packets with hash and reflect to client
	writeChan := make(chan PacketStruct, *chanCap)

    // Create a pool to store all packets received from the client
    // A pool is safe for concurrent use
    var pool = sync.Pool{
                    New: func()interface{} {
                        return &PacketStruct{}
                    }}

	// Create counters for packets sent and received
	packetsRecvCounter := 0
	packetsSentCounter := 0

	// Create wait group to wait for all goroutines to finish before terminating
	var wg sync.WaitGroup
	wg.Add(3)

	// Call these goroutines to handle reads and writes over UDP connection and communicate with HTTP backend over TCP
    go recvPacket(udpConn, readTimeLimit, &packetsRecvCounter, &pool, doneChan, &wg)
	go hashPacket(backendClient, hashURL, shutdownURL, &pool, doneChan, writeChan, *numConcurrentJobs, &wg)
	go reflectPacket(udpConn, writeTimeLimit, &packetsSentCounter, writeChan, &wg)

    // Wait for all goroutines to finish
	wg.Wait()

    log.Println("Packets Received from client: ", strconv.Itoa(packetsRecvCounter))
	log.Println("Packets Sent to client: ", strconv.Itoa(packetsSentCounter))
	log.Println("All done!")
}
