package main

import (
	"log"
	"net"
	"net/http"
	"encoding/json"
    "io/ioutil"
	"bytes"
	"time"
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
}

func hashPacket(client *http.Client, hashURL string, shutdownURL string, recvIn <-chan PacketStruct, writeOut chan<- PacketStruct, wg *sync.WaitGroup) {
	// Close wait group when done
	defer wg.Done()

	// Loop over packets and make calls to HTTP backend to calculate hash of packets
	// Append the hash to the packet and write it to a channel to be reflected back to the client
	hashLoop:
		for {
			select {
			case packet, ok := <- recvIn:
				// If the channel has been closed and drained, exit the outer loop
				if !ok {
					break hashLoop
				} else {
					// Marshal the packet's payload
					requestBody, err := json.Marshal(packet.Packet)
				    if err != nil {
				        log.Fatal(err)
				    }
				    // Create a new HTTP GET Request for the /hash endpoint
				    request, err := http.NewRequest("GET", hashURL, bytes.NewBuffer(requestBody))
				    request.Header.Set("Content-type", "application/json")
				    if err != nil {
				        log.Fatal(err)
				    }
				    // Send the request and acquire a response
				    resp, err := client.Do(request)
				    if err != nil {
				        log.Fatal(err)
				    }
				    // Close the body of the response after done with this iteration
				    defer resp.Body.Close()
				    // Read through the body of the response
				    body, err := ioutil.ReadAll(resp.Body)
				    if err != nil {
				        log.Fatal(err)
				    }
				    // Unmarshal the hash into a byte slice
				    buffer := make([]byte, 8)
				    err = json.Unmarshal(body, &buffer)
				    if err != nil {
				        log.Fatal(err)
				    }
				    // Append the hash to the end of the packet's payload
				    packet.Packet = append(packet.Packet[:], buffer[:]...)

				    // Write the packet to the out channel to be reflected back to the client
				    writeOut <- packet
				}
			}
		}

	// Close the channel when done hashing the packets
	close(writeOut)

	// Create a request to shutdown the HTTP backend server
	request, err := http.NewRequest("GET", shutdownURL, nil)
	if err != nil {
        log.Fatal(err)
    }
    // Send the request and output the response
    resp, err := client.Do(request)
    if err != nil {
        log.Fatal(err)
    }
    defer resp.Body.Close()
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        log.Fatal(err)
    } else {
        log.Println(string(body))
    }

    // Close all idle connections for the HTTP backend client
    client.CloseIdleConnections()
}

// Receives a packet on the UDP connection until no longer receiving a response from a client
// Sends all packets received to a channel to reflect them back to the client
func recvPacket(conn *net.UDPConn, readTimeLimit time.Duration, packetsRecvCounter *int, recvIn chan<- PacketStruct, wg *sync.WaitGroup) {
	// Close wait group when done
	defer wg.Done()

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
				// Send packet to channel for reflecting back to client
				recvIn <- PacketStruct{buffer[:n], addr}
				// Increment the counter for number of packets received
				*packetsRecvCounter++
			}

		}
	// Close the channel when done receiving messages from UDP client
	close(recvIn)
}

// Main function to set up a UDP server that listens and sends back any packets to the client
func main() {
	// Command line args
	var backendHostName = flag.String("backend_host", "http://localhost", "IPv4 of the HTTP backend server (i.e. http://xxx.xx.xx.xx)")
	var backendPortNum = flag.String("backend_port", "8080", "Port number of the HTTP backend server (i.e. 80)")
	// var backendHashEndpoint = flag.String("backend_hash", "/hash", "The hash endpoint of the HTTP backend server (i.e. hash)")
	// var backendShutdownEndpoint = flag.String("backend_shutdown", "/shutdown", "The hash endpoint of the HTTP backend server (i.e. shutdown)")
	var portNum = flag.String("port", "40000", "Port number of the server (i.e. 40000)")
	var wTimeLimit = flag.Int("w_time", 5, "Max number of seconds the connection will wait on a full send queue to free up to send a packet (i.e. 5)")
	var rTimeLimit = flag.Int("r_time", 10, "Max number of seconds the server will wait to receive a request from client before closing (i.e. 10)")
	var chanCap = flag.Int("buffer", 5000, "The max buffer size of the channel used to store received packets that are reflected to client (i.e. 1000)")
	flag.Parse()

	// Define the HTTP backend server address
	backendService := *backendHostName + ":" + *backendPortNum
	hashURL := backendService + "/hash"
	shutdownURL := backendService + "/shutdown"

	// Create a transport for the HTTP client
	tr := &http.Transport {
        DisableKeepAlives: false,
        MaxIdleConnsPerHost: 100,
        MaxConnsPerHost: 0,
        WriteBufferSize: 100,
        ReadBufferSize: 8,
    }

	// Create a client with a specific transport
	backendClient := &http.Client{Transport: tr}

	// Define the server address
	// No host provided so that ResolveUDPAddr resolves to the addreess of UDP end point
	service := ":" + *portNum
	networkName := "udp4"

	// Get address of UDP end point
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
	readTimeLimit := (time.Duration(*rTimeLimit) * time.Second)
	// Set a write deadline for how long should wait on a full send queue to free up to send a packet
	writeTimeLimit := (time.Duration(*wTimeLimit) * time.Second)

	// Create channel to hold packets received from client to calculate hash of
	readChan := make(chan PacketStruct, *chanCap)
	// Create channel to hold packets with hash and reflect to client
	writeChan := make(chan PacketStruct, *chanCap)

	// Create counters for packets sent and received
	packetsRecvCounter := 0
	packetsSentCounter := 0

	// Create waitgroup to wait for all goroutines to finish before terminating
	var wg sync.WaitGroup
	wg.Add(3)

	// Call these goroutines to handle reads and writes over UDP connection
	go recvPacket(udpConn, readTimeLimit, &packetsRecvCounter, readChan, &wg)
	go hashPacket(backendClient, hashURL, shutdownURL, readChan, writeChan, &wg)
	go reflectPacket(udpConn, writeTimeLimit, &packetsSentCounter, writeChan, &wg)
	// Wait for all goroutines to finish
	wg.Wait()
	log.Println("Packets Received from client: ", strconv.Itoa(packetsRecvCounter))
	log.Println("Packets Sent to client: ", strconv.Itoa(packetsSentCounter))
	log.Println("All done!")
}
