# udp_client_server
This project implements a simple client and server that communicate over a UDP connection using Go.
The client sends packets with a payload of 100 bytes to the server, and the server reflects back any packets received back to the client. The client outputs the total number of packets sent to and received from the server.

## System Requirements
This project was tested on Ubuntu 18.04 LTS 64 bit.
Golang version 1.15 is needed to run this project. You can download Golang from [here](https://golang.org/). 

## How to Run
### Server
To run the server, execute `server.sh` from the command line (i.e. `./server.sh`).
There are some optional positional arguments that can be configured:
1. `port`  Port number of the server (i.e. 40000)
2. `w_time` Max number of seconds the connection will wait on a full send queue to free up to send a packet (i.e. 5)
3. `r_time` Max number of seconds the server will wait to receive a request from client before closing (i.e. 10)
4. `buffer` The max buffer size of the channel used to store received packets that are reflected to client (i.e. 1000)

### Client 
To run the client, you need the IPv4 address of the server.
Execute `client.sh` from the command line, followed by the server's IPv4 (i.e. `./client.sh -h 169.254.105.13`).
There are some optional positional arguments that can be configured:
1. `port` Port number of host to connect to (i.e. 40000)
2. `c_time` Number of minutes the connection with the server will stay alive for (i.e. 10)
3. `r_time` Max number of milliseconds the client will attempt to spend waiting to read from server (i.e. 500)
4. `buffer` The max buffer size of the channels used to record packets sent and received (i.e. 1000)

