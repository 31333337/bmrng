#!/usr/bin/env bash
set -xe

tmpdir=$(mktemp -d)
scriptdir=$(cd $(dirname $0); pwd -P)
sourcedir=$(cd $scriptdir/..; pwd -P)

# Check for clang++
if ! command -v clang++ &> /dev/null; then
    echo "clang++ is not installed or not in PATH."
    exit 1
fi

cd $tmpdir
git clone https://github.com/herumi/mcl
cd mcl/
# Locate the libmclbn384_256.so.1 shared library
LIB_PATH=$(find / -name libmclbn384_256.so.1 2>/dev/null)

if [ -z "$LIB_PATH" ]; then
    echo "Error: libmclbn384_256.so.1 not found!"
    exit 1
fi

LIB_DIR=$(dirname "$LIB_PATH")

# Add the library's directory to LD_LIBRARY_PATH
if ! echo $LD_LIBRARY_PATH | grep -q "$LIB_DIR"; then
    echo "Adding $LIB_DIR to LD_LIBRARY_PATH"
    export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:$LIB_DIR

    # Make this change permanent
    echo "export LD_LIBRARY_PATH=$LD_LIBRARY_PATH" >> ~/.bashrc
fi

# Refresh the shared library cache
sudo ldconfig

# Consider uncommenting the next line if you want to use a specific version
# git checkout 3130df5 #herumi/mcl v1.52

# Check for Apple Silicon in Docker
arch=$(uname -m)
os=$(uname -o)

if [[ "$arch" == "aarch64" && "$os" == "GNU/Linux" ]]; then
    echo "Running on Apple Silicon (aarch64) in Docker"
    apt install cmake openssl gcc -ys 
    # Insert rules or commands specific to this environment
    # e.g., Different compiler flags, dependencies, etc.
fi

cmake -S . -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_CXX_COMPILER=clang++
cmake --build build
sudo cmake --build build --target install

if [[ "$arch" == "x86" && "$(uname)" = "Linux" ]]; then
    echo "Building for Linux"
    apt install gcc g++ libgmp3-dev cmake openssl-dev
    # Consider uncommenting the next line if ldconfig is required after installation
    # sudo ldconfig
fi

# Uncomment and adjust the following lines if you plan to use them
# cd $sourcedir/
# GOPATH=$sourcedir go get github.com/stretchr/testify/assert
# GOPATH=$sourcedir go get github.com/Sirupsen/logrus
# GOPATH=$sourcedir go get github.com/dustin/go-humanize
