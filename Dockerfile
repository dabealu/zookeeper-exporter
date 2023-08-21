FROM ubuntu:latest as builder

ARG GO_VERSION
ENV GO_VERSION=${GO_VERSION}

RUN apt-get update
RUN apt-get install -y wget git gcc

RUN wget -P /tmp "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz"

RUN tar -C /usr/local -xzf "/tmp/go${GO_VERSION}.linux-amd64.tar.gz"
RUN rm "/tmp/go${GO_VERSION}.linux-amd64.tar.gz"

ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH
RUN mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"

WORKDIR $GOPATH

WORKDIR     /usr/src/zookeeper-exporter
COPY        . /usr/src/zookeeper-exporter
RUN         go build -v 

FROM ubuntu:latest
COPY        --from=builder /usr/src/zookeeper-exporter/zookeeper-exporter /usr/local/bin/zookeeper-exporter
ENTRYPOINT  ["/usr/local/bin/zookeeper-exporter"]
