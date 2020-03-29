FROM        golang:1.14-alpine as builder
WORKDIR     /usr/src/zookeeper-exporter
COPY        . /usr/src/zookeeper-exporter
RUN         go build -v 

FROM        alpine:3.11
COPY        --from=builder /usr/src/zookeeper-exporter/zookeeper-exporter /usr/local/bin/zookeeper-exporter
ENTRYPOINT  ["/usr/local/bin/zookeeper-exporter"]
