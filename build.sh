#!/bin/bash -e

# build binary
docker run -ti --rm \
    -v $PWD:/usr/src/zookeeper-exporter \
    -w /usr/src/zookeeper-exporter \
    golang:1.9-alpine /bin/sh -c 'go build -v'

# build docker image
docker build -t ${1:-'zookeeper-exporter:latest'} .

rm -f zookeeper-exporter