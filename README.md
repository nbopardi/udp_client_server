# udp_client_server
This project implements a simple client and server that communicate over a UDP connection using Go.
The client sends packets with a payload of 100 bytes to the server. The server receives packets and makes a call to an HTTP backend server over TCP to calculate the fnv1a hash of the packet's payload. The server then appends the hash to the end of the packet's payload and sends it back to the client.
The client outputs the total number of packets sent to and received from the server, and the server outputs the total number of packets received from and sent to the client.

## System Requirements
This project was tested on Ubuntu 18.04 LTS 64 bit.
Golang version 1.15 is needed to run this project. You can download Golang from [here](https://golang.org/). 

## How to Run
### 1) HTTP Backend
To run the backend, execute `backend.sh` from the command line (i.e. `./backend.sh`).
There are some optional positional arguemnts that can be configured:
1. `port` Port number of the HTTP backend server (default: 80)
2. `rh_time` Max number of seconds the HTTP backend server entire will spend reading the headers of the request (default: 20)
3. `w_time` Max number of seconds the HTTP backend server will wait before timing out writes of the response (default: 20)

### 2) UDP Server
To run the server, you need the IPv4 address of the HTTP backend server.
Execute `server.sh` from the command line, followed by the backend's IPv4 (i.e. `./server.sh -b_host 167.173.192.231`).
There are some optional positional arguments that can be configured:

1. `b_port` Port number of the HTTP backend server (default: 80)
2. `port`  Port number of the server (default: 40000)
3. `w_time` Max number of seconds the connection will wait on a full send queue to free up to send a packet (default: 5)
4. `r_time` Max number of seconds the server will wait to receive a request from client before closing (default: 10)
5. `n_jobs` Max number of requests made to the HTTP backend occurring at the same time (default: 15000)
6. `ec_time` Amount of seconds to wait for the HTTP backend's first response headers after fully writing the request headers (default: 4)
7. `rh_time` Amount of seconds to wait for the HTTP backend's response headers after fully writing the request and body (default: 10)
8. `ic_time` Max amount of seconds an idle (keep-alive) connection will remain idle before closing itself (default: 10)
9. `iconn_host` Max idle (keep-alive) connections to keep per-host (default: 10000)
10. `buffer` The max buffer size of the channel used to store received packets that are reflected to client (default: 1000000)

### 3) UDP Client 
To run the client, you need the IPv4 address of the UDP server.
Execute `client.sh` from the command line, followed by the server's IPv4 (i.e. `./client.sh -host 169.254.105.13`).
There are some optional positional arguments that can be configured:
1. `port` Port number of host to connect to (default: 40000)
2. `c_time` Number of minutes the connection with the server will stay alive for (default: 10)
3. `buffer` The max buffer size of the channels used to record packets sent and received (default: 1000000)

## Optimizations
### 1) Using a rate limiter for the UDP server to communicate with the HTTP backend
Spawning a goroutine to communicate with the HTTP backend for each incoming packet overwhelmed the HTTP server and caused out of memory errors. By instituting a rate limiter, only N goroutines (`n_jobs`) would be active at once to make calls to the backend. This solves the former problems, but ultimately sacrifices the number of packets that are sent back to the client. Configuring `n_jobs` in accordance to the number of CPUs available is vital for sending more packets back to the client.

### 2) Using [sync.Pool](https://golang.org/pkg/sync/#Pool.Get) for receiving packets on the UDP server
The previous implementation of this project had the UDP server reflect all received packets back to the client by using channels for transfering packets from the receive to the send goroutines. Now that the server has to communicate with the backend over TCP, the channel to store incoming packets would fill up, which blocks the UDP server from receiving packets. By inputting received packets into a pool, which is a thread-safe, dynamically-sized data structure, the UDP server can continue to receive packets without issues. This substantially increased the number of packets being sent back to the client.

### 3) System tuning and resource limitations
Adjusting linux kernel parameters using sysctl ensured that there were no limitations on the number of file descriptors, connections backlog, allocatable buffer-space, buffer size, etc. There are also optimizations specifically for UDP and TCP connections. Setting these parameters ensures that the goroutines are able to run at full capacity.

### 4) Pin main goroutines to OS threads
For the UDP server, the goroutines used to communicate with the HTTP backend were competing over the same CPU resources that the receive, hash, and send goroutines also used, which slowed down the UDP server's throughput. In order to prioritize the main goroutines (receive, hash, and send), each main goroutine was locked to its own OS thread, leaving the remaining worker goroutines to compete amongst themselves for CPU resouces. This provided a boost in the number of packets received and sent by the UDP server.
