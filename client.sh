#!/bin/sh

# This shell script sets up the UDP client

# Read in command line arguments for client
helpFunction()
{
   echo ""
   echo "Usage: $0 -host hostName -port portNum -c_time connectionTime -r_time readTime -buffer channelBufferSize"
   echo "\t-host IPv4 of host to connect to (i.e. 169.254.105.13)"
   echo "\t-port Port number of host to connect to (i.e. 40000)"
   echo "\t-c_time Number of minutes the connection with the server will stay alive for (i.e. 10)"
   echo "\t-r_time Max number of milliseconds the client will attempt to spend waiting to read from server (i.e. 500)"
   echo "\t-buffer The max buffer size of the channels used to record packets sent and received (i.e. 1000)"
   exit 1 # Exit script after printing help
}


# Set default values for all arguments
hostName="localhost"
portNum="40000"
c_time=10
r_time=500
buffer=1000

if [ $# -eq 0 ] ; then
	echo "Host name required as positional argument 1. Aborting";
	helpFunction
else

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
			-h|-host) hostName="$2"; shift ;;
			-p|-port) portNum="$2"; shift ;;
			-c|-c_time) c_time="$2"; shift ;;
			-r|-r_time) r_time="$2"; shift ;;
			-b|-buffer) buffer="$2"; shift ;;
			*) echo "Unknown parameter passed: $1"; helpFunction ;;
		esac
		shift
	done

	# Run udp_client.go with positional args
	go run ./udp_client.go -host="$hostName" -port="$portNum" -c_time="$c_time" -r_time="$r_time" -buffer="$buffer"
fi
