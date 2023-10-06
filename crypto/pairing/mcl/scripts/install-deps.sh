#!/usr/bin/env bash

set -xe

tmpdir=`mktemp -d`

scriptdir=$(cd $(dirname $0); pwd -P)
sourcedir=$(cd $scriptdir/..; pwd -P)
. $scriptdir/shlibs/os.sh

cd $tmpdir
git clone https://github.com/herumi/mcl
cd mcl/
git checkout 3130df5 #herumi/mcl v1.52
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build
sudo cmake --build build --target install
if [ "$OS" = "Linux" ]; then
    sudo ldconfig
## on M1 there is not much info on how to replace ldconfig
## it's deprecated and might not be needed in Darwin, MacOS's case.
elif [ "$OS" = "Darwin" ]; then
    sudo update_dyld_shared_cache
fi

#(
#    cd $sourcedir/
#    GOPATH=$sourcedir go get github.com/stretchr/testify/assert
#    GOPATH=$sourcedir go get github.com/Sirupsen/logrus
#    GOPATH=$sourcedir go get github.com/dustin/go-humanize
#)
