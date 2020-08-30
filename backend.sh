#!/bin/sh

# This shell script sets up the HTTP backend

# Read in command line arguments for backend
helpFunction()
{
   echo ""
   echo "Usage: $0 -port portNum -rh_time readHeaderTime -w_time writeTime"
   echo "\t-port Port number of the HTTP backend server (i.e. 80)"
   echo "\t-rh_time Max number of seconds the HTTP backend server entire will spend reading the headers of the request (i.e. 20)"
   echo "\t-w_time Max number of seconds the HTTP backend server will wait before timing out writes of the response (i.e. 20)"
   exit 1 # Exit script after printing help
}

# Set default values for all arguments
portNum="80"
rh_time=20
w_time=20

# Verify that golang installed
if ! [ -x "$(command -v go)" ]; then
    echo "golang is not installed. Please install go1.15. Aborting"
    exit 1
fi

# System level optimizations for TCP

# Increase number of incoming connections
sysctl -w net.core.somaxconn=31457280

# Increase number of incoming connections backlog
sysctl -w net.core.netdev_max_backlog=31457280

# Increase the maximum amount of option memory buffers
sysctl -w net.core.optmem_max=31457280

# Increase the maximum total buffer-space allocatable
# This is measured in units of pages (4096 bytes)
sysctl -w net.ipv4.tcp_mem='26777216 26777216 26777216'

# Increase receive buffer size
sysctl -w net.core.rmem_default=31457280
sysctl -w net.core.wmem_default=31457280

# Increase these as well
sysctl -w net.core.rmem_max=33554432
sysctl -w net.core.wmem_max=33554432

# Increase TCP read and write buffers
sysctl -w net.ipv4.tcp_rmem='33554432 33554432 33554432'
sysctl -w net.ipv4.tcp_wmem='33554432 33554432 33554432'
sysctl -w net.ipv4.tcp_mem='33554432 33554432 33554432'
sysctl -w net.ipv4.route.flush=1

# Reuse TIME_WAIT and TIMEOUT TCP sockets
echo 1 > /proc/sys/net/ipv4/tcp_tw_reuse
echo 5 > /proc/sys/net/ipv4/tcp_fin_timeout

# Increase all params to max
ulimit -u unlimited
# Increase max number of open files to 614400
prlimit --pid=$PPID --nofile=614400

# Parse through command line args
while [ "$#" -gt 0 ]; do
    case $1 in
        -p|-port) portNum="$2"; shift ;;
        -rh|-rh_time) rh_time="$2"; shift ;;
        -w|-w_time) w_time="$2"; shift ;;
        *) echo "Unknown parameter passed: $1"; helpFunction ;;
    esac
    shift
done

# Run http_backendgo with positional args
go run ./http_backend.go -port="$portNum" -rh_time="$rh_time" -w_time="$w_time"

