FROM        alpine:3.6
COPY        zookeeper-exporter /usr/local/bin/zookeeper-exporter
ENTRYPOINT  ["/usr/local/bin/zookeeper-exporter"]
