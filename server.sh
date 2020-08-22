#!/bin/sh

# This shell script sets up the UDP server

# Read in command line arguments for server
helpFunction()
{
	 echo ""
	 echo "Usage: $0 -port portNum -w_time writeTime -r_time readTime -buffer channelBufferSize"
	 echo "\t-port Port number of the server (i.e. 40000)"
	 echo "\t-w_time Max number of seconds the connection will wait on a full send queue to free up to send a packet (i.e. 5)"
	 echo "\t-r_time Max number of seconds the server will wait to receive a request from client before closing (i.e. 10)"
	 echo "\t-buffer The max buffer size of the channel used to store received packets that are reflected to client (i.e. 1000)"
	 exit 1 # Exit script after printing help
}


# Set default values for all arguments
portNum="40000"
w_time=5
r_time=10
buffer=5000

# Verify that golang installed
if ! [ -x "$(command -v go)" ]; then
		echo "golang is not installed. Please install go1.15. Aborting"
		exit 1
fi

# System level optimizations for UDP
# Increase UDP receive buffer size
sysctl -w net.core.rmem_default=31457280
sysctl -w net.core.wmem_default=31457280

# Increase UDP max/mins as well
sysctl -w net.core.rmem_max=33554432
sysctl -w net.core.wmem_max=33554432
sysctl -w net.ipv4.udp_rmem_min=16384
sysctl -w net.ipv4.udp_wmem_min=16384

# Parse through command line args
while [ "$#" -gt 0 ]; do
		case $1 in
				-p|-port) portNum="$2"; shift ;;
				-w|-w_time) w_time="$2"; shift ;;
				-r|-r_time) r_time="$2"; shift ;;
				-b|-buffer) buffer="$2"; shift ;;
				*) echo "Unknown parameter passed: $1"; helpFunction ;;
		esac
		shift
done

# Run udp_server.go with positional args
go run ./udp_server.go -port="$portNum" -w_time="$w_time" -r_time="$r_time" -buffer="$buffer"
