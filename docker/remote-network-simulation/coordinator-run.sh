#!/bin/bash

set -ex
BASE_DIR="/src"

if [[ $(pwd) == "/com.docker.devenvironments.code/"* ]]; then
    BASE_DIR="/com.docker.devenvironments.code"
fi

# Fetch IP address of eth0
ETH0_IP=$(ip addr show eth0 | grep "inet\b" | awk '{print $2}' | cut -d/ -f1)

# Create 10 hosts on 10 different ports based on eth0 IP and append to hostsfile
START_PORT=8000
END_PORT=$((START_PORT + 9))
hostsfile="$BASE_DIR/go/trellis/cmd/experiments/ip.list"
> "${hostsfile}" # Emptying the file before appending
for port in $(seq $START_PORT $END_PORT); do
    echo "${ETH0_IP}:${port}" >> "${hostsfile}"
done

args='--numservers 3 --numgroups 3 --numusers 10 --groupsize 3 --numlayers 10'
cd "$BASE_DIR/go/trellis/cmd/coordinator" && go build
cd "$BASE_DIR/go/0kn/cmd/xtrellis" && go build && ./xtrellis coordinator config ${args} --hostsfile ${hostsfile}
cd "$BASE_DIR/go/trellis/cmd/coordinator" && ./coordinator ${args} --runtype 2
