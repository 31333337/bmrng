#!/bin/bash

set -ex
BASE_DIR="/src"

if [[ $(pwd) == "/com.docker.devenvironments.code/"* ]]; then
    BASE_DIR="/com.docker.devenvironments.code"
fi

args='--numservers 3 --numgroups 3 --numusers 10 --groupsize 3 --numlayers 10'
hostsfile="$BASE_DIR/go/trellis/cmd/experiments/ip.list"
echo -e 'server-0\nserver-1\nserver-2' > "${hostsfile}"
cd "$BASE_DIR/go/trellis/cmd/coordinator" && go build
cd "$BASE_DIR/go/0kn/cmd/xtrellis" && go build && ./xtrellis coordinator config ${args} --hostsfile ${hostsfile}
cd "$BASE_DIR/go/trellis/cmd/coordinator" && ./coordinator ${args} --runtype 2
# create hosts file, generate config from it, launch coordinator using it

#iargs='--numservers 3 --numgroups 3 --numusers 10 --groupsize 3 --numlayers 10'
#hostsfile="$BASE_DIR/go/trellis/cmd/experiments/ip.list"

#echo -e 'server-0\nserver-1\nserver-2' > "${hostsfile}"
#cd /com.docker.devenvironments.code/go/trellis/cmd/coordinator && go build
#cd /com.docker.devenvironments.code/go/0kn/cmd/xtrellis && go build && ./xtrellis coordinator config ${args} --hostsfile ${hostsfile}
#cd /com.docker.devenvironments.code/go/trellis/cmd/coordinator && ./coordinator ${args} --runtype 2
