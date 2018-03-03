### Prometheus zookeeper exporter
Exports `mntr` zookeeper's stats in prometheus format.  
`zk_followers`, `zk_synced_followers` and `zk_pending_syncs` metrics are available only on cluster leader.  

#### Build
`./build.sh` script builds `zookeeper-exporter:latest` docker image.  
To build image with different name, pass it to `build.sh` as a first arg.  

#### Usage
```
Usage of zookeeper-exporter:
  -listen string
        address to listen on (default "0.0.0.0:8080")
  -location string
        metrics location (default "/metrics")
  -zk-host string
        zookeeper host (default "127.0.0.1")
  -zk-port string
        zookeeper port (default "2181")
```

An example `docker-compose.yml` can be used for management of clustered zookeeper + exporters:
```
# start zk cluster and exporters
docker-compose up -d

# get metrics of first exporter (second and third exporters are on 8082 and 8083 ports)
curl -s localhost:8081/metrics

# shutdown containers
docker-compose down -v
```