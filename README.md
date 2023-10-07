# Trellis: Fast and Scalable Metadata Private Anonymous Broadcast

**Paper:** https://eprint.iacr.org/2022/1548.pdf (NDSS 2023)
The following instructions are for an AWS amazon linux 2 AMI

# Install Guide

## Dependencies
this repo and Go `1.20` or later

### On Ubuntu
First install your dependencies
```sh
sudo apt install gcc g++ libgmp3-dev cmake openssl-dev
```
### On OSX
```sh
brew install gmp cmake openssl
```

## Building the Commands

First the crypto library
```sh
cd crypto/pairing/mcl/scripts
export CC=gcc
export CXX=g++
./install-deps.sh
```
Then the commands themselves
```sh
( cd cmd/server && go install && go build )
( cd cmd/client && go install && go build )
( cd cmd/coordinator && go install && go build )
```

# Usage Guide
Basic test
```
./cmd/coordinator --numusers 100 --numservers 10 --numlayers 10 --groupsize 3 --numgroups 3 --runtype 0
./cmd/coordinator --numusers 100 --numservers 10 --numlayers 10 --groupsize 3 --numgroups 3 --runtype 1
```
### Parameters
| argument   | meaning                                         |
| ---------- | ----------------------------------------------- |
| f          | fraction of servers controlled by the adversary |
| numservers | total number of servers                         |
| numusers   | number of messages                              |
| numlayers  | (optional) number of layers                     |
| groupsize  | (optional) size of anytrust group               |
| numgroups  | (optional) number of anytrust groups            |
| runtype    | 0: create keys, 1: run local, 2: run on servers |

Additional arguments will be computed based on the provided values, but you can provide an override for them, for example, to use a simulated number of bins.

<!-- ### Helper files
Helper files (may need modification for your aws account)
| file                 | purpose                                        |
| -------------------- | ---------------------------------------------- |
| aws_global_setup.py  | setup private vpn network                      |
| aws_launch.py        | launch test in one aws region                  |
| aws_global_launch.py | launch test in multiple aws regions            |
| aws_bandwidth.py     | limit the bandwidth of each machine            |
| aws_latency.py       | add (artificial) network delay to each machine |
| aws_terminate.py     | kill all the machines with the specified key   | --> |


<!-- ### Other programs 
Run key exchange in ```server/keyExchange```
``` go test exchangeKey_test.go ```
Calculate the number of bins empirically (for 1/256 probability of failure)
In ```cmd/simulation```
| argument   | meaning                 |
| ---------- | ----------------------- |
| numservers | total number of servers |
| numusers   | number of messages      |
| numlayers  | number of layers        |
| numtrials  | number of trials        |
Remember to then add additional layers to account for failure probability.





 -->
