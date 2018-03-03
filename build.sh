#!/bin/bash -e
docker build -t ${1:-'dabealu/zookeeper-exporter:latest'} .