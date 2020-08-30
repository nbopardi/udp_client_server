#!/bin/sh

# This shell script sets up the UDP server

# Read in command line arguments for server
helpFunction()
{
	echo ""
	echo "Usage: $0 -b_host backendHostName -b_port backendPortNum -port portNum -w_time writeTime -r_time readTime -n_jobs numConcurrentJobs -ec_time expectContTime -rh_time respHeaderTime -ic_time idleConnTime -iconn_host idleConnsPerHost -buffer channelBufferSize"
	echo "\t-b_host IPv4 of the HTTP backend server (i.e. 169.254.105.13)"
	echo "\t-b_port Port number of the HTTP backend server (i.e. 80)"
	echo "\t-port Port number of the server (i.e. 40000)"
	echo "\t-w_time Max number of seconds the connection will wait on a full send queue to free up to send a packet (i.e. 5)"
	echo "\t-r_time Max number of seconds the server will wait to receive a request from client before closing (i.e. 10)"
	echo "\t-n_jobs Max number of requests made to the HTTP backend occurring at the same time (i.e. 15000)"
	echo "\t-ec_time Amount of seconds to wait for the HTTP backend's first response headers after fully writing the request headers (i.e. 4)"
	echo "\t-rh_time Amount of seconds to wait for the HTTP backend's response headers after fully writing the request and body (i.e. 10)"
	echo "\t-ic_time Max amount of seconds an idle (keep-alive) connection will remain idle before closing itself (i.e. 10)"
	echo "\t-iconn_host Max idle (keep-alive) connections to keep per-host (i.e. 10000)"
	echo "\t-buffer The max buffer size of the channel used to store received packets that are reflected to client (i.e. 1000000)"
	exit 1 # Exit script after printing help
}


# Set default values for all arguments
b_host="localhost"
b_port=80
portNum="40000"
w_time=5
r_time=10
n_jobs=15000
ec_time=4
rh_time=10
ic_time=10
iconn_host=10000
buffer=1000000


if [ $# -eq 0 ] ; then
	echo "Backend host name required as positional argument 1. Aborting";
	helpFunction
else
	# Verify that golang installed
	if ! [ -x "$(command -v go)" ]; then
			echo "golang is not installed. Please install go1.15. Aborting"
			exit 1
	fi

	# System level optimizations for UDP and TCP

	# Increase number of incoming connections
	sysctl -w net.core.somaxconn=31457280

	# Increase number of incoming connections backlog
	sysctl -w net.core.netdev_max_backlog=31457280

	# Increase the maximum amount of option memory buffers
	sysctl -w net.core.optmem_max=31457280

	# Increase the maximum total buffer-space allocatable
	# This is measured in units of pages (4096 bytes)
	sysctl -w net.ipv4.tcp_mem='26777216 26777216 26777216'
	sysctl -w net.ipv4.udp_mem='26777216 26777216 26777216'

	# Increase udp receive buffer size
	sysctl -w net.core.rmem_default=31457280
	sysctl -w net.core.wmem_default=31457280

	# Increase these as well
	sysctl -w net.core.rmem_max=33554432
	sysctl -w net.core.wmem_max=33554432
	sysctl -w net.ipv4.udp_rmem_min=31457280
	sysctl -w net.ipv4.udp_wmem_min=31457280

	# Increase TCP read and write buffers
	sysctl -w net.ipv4.tcp_rmem='33554432 33554432 33554432'
	sysctl -w net.ipv4.tcp_wmem='33554432 33554432 33554432'
	sysctl -w net.ipv4.tcp_mem='33554432 33554432 33554432'
	sysctl -w net.ipv4.route.flush=1

	# Reuse TIME_WAIT and TIMEOUT TCP sockets
	echo 1 > /proc/sys/net/ipv4/tcp_tw_reuse
	echo 5 > /proc/sys/net/ipv4/tcp_fin_timeout

	# Increase max number of open files to 614400
	prlimit --pid=$PPID --nofile=614400

	# Parse through command line args
	while [ "$#" -gt 0 ]; do
			case $1 in
					-b|-b_host) b_host="$2"; shift ;;
					-bp|-b_port) b_port="$2"; shift ;;
					-p|-port) portNum="$2"; shift ;;
					-w|-w_time) w_time="$2"; shift ;;
					-r|-r_time) r_time="$2"; shift ;;
					-n|-n_jobs) n_jobs="$2"; shift ;;
					-ec|-ec_time) ec_time="$2"; shift ;;
					-rh|-rh_time) rh_time="$2"; shift ;;
					-ic|-ic_time) ic_time="$2"; shift ;;
					-ih|-iconn_host) iconn_host="$2"; shift ;;
					-b|-buffer) buffer="$2"; shift ;;
					*) echo "Unknown parameter passed: $1"; helpFunction ;;
			esac
			shift
	done

	# Run udp_server.go with positional args
	go run ./udp_server.go -backend_host="$b_host" -backend_port="$b_port" -port="$portNum" -w_time="$w_time" -r_time="$r_time" -n_jobs="$n_jobs" -ec_time="$ec_time" -rh_time="$rh_time" -ic_time="$ic_time" -iconn_host="$iconn_host" -buffer="$buffer"
fi
