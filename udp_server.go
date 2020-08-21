package main

import (
	"log"
	"net"
	"time"
	"sync"
	"strconv"
	"flag"
)

// Send a given packet back to the client
func reflectPacket(conn *net.UDPConn, addr *net.UDPAddr, messg []byte, writeTimeLimit time.Duration, packetsSentCounter *int, wg *sync.WaitGroup) {
	// Close wait group when done
    defer wg.Done()

    // Set a deadline for how long server should wait to write message
	err := conn.SetWriteDeadline(time.Now().Add(writeTimeLimit))
	if err != nil {
		log.Println("Could not set write deadline for connection: ", err)
	}

	// Reflect the message back to the client
	_, err = conn.WriteToUDP(messg, addr)

	// Error handling
	if err != nil {
		log.Println("Could not write message to UDP client: ", err)
	} else {
		// Increment the counter for the number of packets sent back
		*packetsSentCounter++
	}
}

// Handles the UDP connection I/O
func handleUDPConnection(conn *net.UDPConn, readTimeLimit time.Duration, writeTimeLimit time.Duration) {
	// Create waitgroup to wait for all goroutines to finish before terminating
	var wg sync.WaitGroup

	// Counter to hold how many packets sent and received
	packetsRecvCounter := 0
	packetsSentCounter := 0

	// Loop to handle reading and writing messages to client
	// Exited when time limit / deadline reached
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
			messgReceived := buffer[:n]

			// Exit from loop if read time limit reached
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
	                    log.Println("Time limit reached")
	                    break receiveSendLoop
	         	}
	         	log.Println("Could not receive message from UDP client: ", err)
				log.Fatal(err)
			}

			// Increment the counter for number of packets received
			packetsRecvCounter++

			// Start a goroutine to write the packet back to the client
			wg.Add(1)
			go reflectPacket(conn, addr, messgReceived, writeTimeLimit, &packetsSentCounter, &wg)
		}

	wg.Wait()
	log.Println("Packets Received from client: ", strconv.Itoa(packetsRecvCounter))
	log.Println("Packets Sent to client: ", strconv.Itoa(packetsSentCounter))
	log.Println("All done!")
}


func main() {
	// Command line args
    var hostName = flag.String("host", "localhost", "IPv4 of host to connect to (i.e. 169.254.105.139")
    var portNum = flag.String("port", "40000", "Port number of host to connect to (i.e. 40000)")
    var wTimeLimit = flag.Int("w_time", 5, "Max number of seconds the connection will wait on a full send queue to free up to send a packet (i.e. 5")
    var rTimeLimit = flag.Int("r_time", 5, "Max number of seconds the server will wait to receive a response from client (i.e. 5)")
    flag.Parse()

	service := *hostName + ":" + *portNum
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

	// Close the UDP connection when done with everything
	defer udpConn.Close()

	// Set a read deadline for how long should wait for client response
	readTimeLimit := (time.Duration(*rTimeLimit) * time.Second)
	// Set a write deadline for how long should wait on a full send queue to free up to send a packet
	writeTimeLimit := (time.Duration(*wTimeLimit) * time.Second)

	log.Println("UDP server up and listening at: ", service)

	// Call this method to handle reads and spawning write goroutines over UDP connection
	handleUDPConnection(udpConn, readTimeLimit, writeTimeLimit)
}
