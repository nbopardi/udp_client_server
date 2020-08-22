package main

import (
	"log"
	"net"
	"time"
	"sync"
	"strconv"
	"flag"
)

// Packet struct that contains:
// 1) a byte slice representing the packet's payload
// 2) an address from the sender of the packet
type Packet struct {
	packet 	[]byte
	addr 	*net.UDPAddr
}

// Reflect packets from a channel back to the client
func reflectPacket(conn *net.UDPConn, writeTimeLimit time.Duration, packetsSentCounter *int, recvOut <-chan Packet, wg *sync.WaitGroup) {
	// Close wait group when done
	defer wg.Done()

	reflectLoop:
		for {
			select {
			case packet, ok := <- recvOut:
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
                   	_, err = conn.WriteToUDP(packet.packet, packet.addr)
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

// Receives a packet on the UDP connection until no longer receiving a response from a client
// Sends all packets received to a channel to reflect them back to the client
func recvPacket(conn *net.UDPConn, readTimeLimit time.Duration, packetsRecvCounter *int, recvIn chan<- Packet, wg *sync.WaitGroup) {
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
						log.Println("Time limit reached for awaiting client request")
						break receiveSendLoop
				}
				log.Fatal("Could not receive message from UDP client: ", err)
			} else {
				// Send packet to channel for reflecting back to client
				recvIn <- Packet{buffer[:n], addr}
				// Increment the counter for number of packets received
				*packetsRecvCounter++
			}

		}
	// Close the channel when done receiving messages from UDP client
	close(recvIn)
}


func main() {
	// Command line args
	var portNum = flag.String("port", "40000", "Port number of the server (i.e. 40000)")
	var wTimeLimit = flag.Int("w_time", 5, "Max number of seconds the connection will wait on a full send queue to free up to send a packet (i.e. 5")
	var rTimeLimit = flag.Int("r_time", 10, "Max number of seconds the server will wait to receive a request from client before closing (i.e. 10)")
	var chanCap = flag.Int("buffer", 5000, "The max buffer size of the channel used to store received packets that are reflected to client (i.e. 1000)")
	flag.Parse()

	// Define the address
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

	// Create channel to hold packets to reflect back to client
	readChan := make(chan Packet, *chanCap)

	// Create counters for packets sent and received
	packetsRecvCounter := 0
	packetsSentCounter := 0

	// Create waitgroup to wait for all goroutines to finish before terminating
	var wg sync.WaitGroup
	wg.Add(2)

	// Call these goroutines to handle reads and writes over UDP connection
	go recvPacket(udpConn, readTimeLimit, &packetsRecvCounter, readChan, &wg)
	go reflectPacket(udpConn, writeTimeLimit, &packetsSentCounter, readChan, &wg)

	// Wait for all goroutines to finish
	wg.Wait()
	log.Println("Packets Received from client: ", strconv.Itoa(packetsRecvCounter))
	log.Println("Packets Sent to client: ", strconv.Itoa(packetsSentCounter))
	log.Println("All done!")
}
