#!/bin/bash
# Script to build PromHouse using a grpc docker container, so you don't have to setup grpc locally.
# NOTE: it has to be run from the root of repo (PromHouse/ directory). 
# You can use make build for that

set -ex

# Build grpc image to compile
docker build -t "promhouse/compile" ./misc/local/

docker run --net=host -v  "${PWD}":/go/src/github.com/hmrc/Promhouse/ promhouse/compile
