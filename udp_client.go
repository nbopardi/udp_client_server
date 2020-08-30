package main

import (
	"log"
	"net"
	"encoding/binary"
	"time"
	"sync"
	"strconv"
	"flag"
)

// Sends packets to a server using the given UDP connection
// Writes the packets to a channel for checking which packets have been received from the server
// This process stops after the connection times out
func sendMessages(conn *net.UDPConn, writeOut chan<- uint32, packetsSentCounter *int, wg *sync.WaitGroup) {
	// Close the wait group once done
	defer wg.Done()

	// Create message counter (unique identifier for sending messages)
	var messgCounter uint32
	messgCounter = 0

	// Loop for writing packets
	// Exited when time limit / deadline reached
	writeLoop:
		for {
			// Create message by placing uint32 into byte slice
			messg := make([]byte, 100)
			binary.LittleEndian.PutUint32(messg, messgCounter)

			// Write message
			_, err := conn.Write(messg)

			// Handle any errors
			if err != nil {
				// Exit from loop if time limit reached
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					log.Println("From Send: Time limit reached")
					break writeLoop
				}
				log.Fatal("Could not send packet to server:", err)
			} else {
				// Write the packet contents to out channel
				writeOut <- messgCounter
				// Increment the packets sent counter
				*packetsSentCounter++
			}
			// Increment the message counter
			messgCounter++
		}

	// Close the channel for sending out written packets
	close(writeOut)
}

// Receives packets from a server using the given UDP connection
// Packets contain a fnv1a hash of the packet's original payload appended to the end
// Writes packets to a channel for checking which packets have been received from the server
// This process stops after the connection times out
func receiveMessages(conn *net.UDPConn, recvOut chan<- []byte, wg *sync.WaitGroup) {
	// Close wait group when done
	defer wg.Done()

	// Loop that runs to receive messages
	// Exited when time limit / deadline reached
	receiveLoop:
		for {
			// Create buffer to read packet into
			// 100 bytes for original payload + 8 bytes for the hash
			buffer := make([]byte, 108)

			// Read the packet and place the payload in buffer
			n, _, err := conn.ReadFromUDP(buffer)

			// Handle any errors
			if err != nil {
				// Exit from loop if time limit reached
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					log.Println("From Receive: Time limit reached")
					break receiveLoop
				}
				log.Fatal("Could not read from UDP server:", err)
			} else {
				// Send the packet to the received out channel
				recvOut <- buffer[:n]
			}
		}

	// Close the channel for sending out received packets
	close(recvOut)
}

// Records all sent packets from the write channel into a set, which uses a map implementation
func countWritten(writeIn <-chan uint32, set map[uint32]bool, setMutex *sync.RWMutex, wg *sync.WaitGroup) {
	// Close the wait group when done
	defer wg.Done()

	writeLoop:
		for {
			select {
			case packetContent, ok := <- writeIn:
				// If the channel has been closed and drained, exit the outer loop
				if !ok {
					break writeLoop
				} else {
					// Add the packet contents to the set
					setMutex.Lock()
					set[packetContent] = true
					setMutex.Unlock()
				}
			default:
				// Do nothing
			}

		}
}

// Records all received packets from the read channel into a set, which uses a map implementation
func countWrittenRecv(recvIn <-chan []byte, set map[uint32]bool, setMutex *sync.RWMutex, packetsRecvCounter *int, packetsRecvButNotSentCounter *int, wg *sync.WaitGroup) {
	// Close wait group when done
	defer wg.Done()

	recvLoop:
		for {
			select {
			case packet, ok := <-recvIn:
				// If the channel is closed and drained, exit the outer loop
				if !ok {
					break recvLoop
				} else {
					// Verify the packet is greater than 100 bytes in length
					// The fnv1a hash is 8 bytes, which should be appended to the packet's original payload
					if len(packet) < 108 {
						log.Printf("Packet is less than 108 bytes in length: %v\n", packet)
					} else {
						// Create uint32 of packet (take only first 100 bytes of packet for comparison)
						intPacket := binary.LittleEndian.Uint32(packet[:100])
						// Verify received packet is in the set
						setMutex.RLock()
						value := set[intPacket]
						setMutex.RUnlock()

						if value {
							// Remove it from the set and increment the counter
							setMutex.Lock()
							delete(set, intPacket)
							setMutex.Unlock()
							// Increment the packets received counter
							*packetsRecvCounter++
						} else {
							// This condition is hit when the packets have been received, but not yet
							// recorded in the set
							// Increment the packets received but not sent counter
							*packetsRecvButNotSentCounter++
						}
					}
				}
			default:
				// Do nothing
			}
		}
}

// Main function to set up a client that sends and received packets to a server over a UDP connection
func main() {
	// Command line args
	var hostName = flag.String("host", "localhost", "IPv4 of host to connect to (i.e. 169.254.105.13)")
	var portNum = flag.String("port", "40000", "Port number of host to connect to (i.e. 40000)")
	var cTimeLimit = flag.Int("c_time", 1, "Number of minutes the connection with the server will stay alive for (i.e. 1)")
	var chanCap = flag.Int("buffer", 1000, "The max buffer size of the channels used to record packets sent and received (i.e. 1000)")
	flag.Parse()

	// Define the address of server
	service := *hostName + ":" + *portNum
	networkName := "udp4"

	// Get address of UDP end point
	remoteAddr, err := net.ResolveUDPAddr(networkName, service)
	if err != nil {
	  log.Fatal(err)
	}

	// Establish UDP connection with server
	// Local address is nil, meaning a local address is automatically chosen
	conn, err := net.DialUDP(networkName, nil, remoteAddr)
	if err != nil {
	  log.Fatal(err)
	}

	// Close the connection with done with everything
	defer conn.Close()

	// Log information about connection
	log.Printf("Established connection to %s \n", service)
	log.Printf("Remote UDP address: %s \n", conn.RemoteAddr().String())
	log.Printf("Local UDP client address: %s \n", conn.LocalAddr().String())

	// Create a set to add all written packets to by using a map
	// This will be used to verify which packets have been received from the server
	// Uses a mutex to ensure reads and writes do not occur at the same time
	set := make(map[uint32]bool)
	setMutex := &sync.RWMutex{}

	// Create various counters for counting packets
	packetsSentCounter := 0
	packetsRecvCounter := 0
	packetsRecvButNotSentCounter := 0

	// Create channels for processing written and received packets
	writeChan := make(chan uint32, *chanCap)
	readChan := make(chan []byte, *chanCap)

	// Set a time limit for how long the connection will stay alive
	totalTimeLimit := time.Duration(*cTimeLimit) * time.Minute
	err = conn.SetDeadline(time.Now().Add(totalTimeLimit))
	if err != nil {
		log.Fatal("Could not set deadline for connection: ", err)
	}

	// Create waitgroup to wait for all goroutines to finish before terminating
	var wg sync.WaitGroup
	wg.Add(4)

	// Call these goroutines to handle sending and receiving packets to server
	go sendMessages(conn, writeChan, &packetsSentCounter, &wg)
	go receiveMessages(conn, readChan, &wg)
	// Call these goroutines to handle counting number of packets sent and received from server
	go countWritten(writeChan, set, setMutex, &wg)
	go countWrittenRecv(readChan, set, setMutex, &packetsRecvCounter, &packetsRecvButNotSentCounter, &wg)

	// Wait for all goroutines to finish
	wg.Wait()
	log.Println("Packets Sent: ", strconv.Itoa(packetsSentCounter))
	log.Println("Packets Received: ", strconv.Itoa(packetsRecvCounter))
	// log.Println("Packets Sent But Not Recv: ", strconv.Itoa(packetsRecvButNotSentCounter))
	log.Println("All done!")
}
