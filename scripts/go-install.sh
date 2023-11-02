#!/usr/bin/env bash
set -xe

# Define the Go version you want to install
GO_VERSION="1.21.3"
arch=$(uname -m)
os=$(uname -o)
download_url=""

if [[ "$arch" == "aarch64" && "$os" == "GNU/Linux" ]]; then
    # For Apple Silicon (Docker on macOS M1)
    download_url="https://golang.org/dl/go${GO_VERSION}.linux-arm64.tar.gz"
elif [ "$arch" == "x86_64" ]; then
    # For x86 Linux
    download_url="https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz"
else
    echo "Unsupported architecture."
    exit 1
fi

# Download Go
tmpdir=$(mktemp -d)
cd $tmpdir
curl -L $download_url -o go.tar.gz

# Remove the existing Go installation and install the new version
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go.tar.gz

# Optionally, add Go binary path to $PATH for all users
echo "export PATH=$PATH:/usr/local/go/bin" | sudo tee -a /etc/profile

# Cleanup
rm -rf $tmpdir

# Feedback
echo "Go ${GO_VERSION} installation completed."
