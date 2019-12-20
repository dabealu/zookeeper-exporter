### Prometheus zookeeper exporter
Exports `mntr` zookeeper's stats in prometheus format.  
`zk_followers`, `zk_synced_followers` and `zk_pending_syncs` metrics are available only on cluster leader.  

#### Build
`./build.sh` script builds `dabealu/zookeeper-exporter:latest` docker image.  
To build image with different name, pass it to `build.sh` as a first arg.  

#### Usage
**Note:** starting from zookeeper v3.4.10 it's required to have `mntr` command whitelisted (details: [4lw.commands.whitelist](https://zookeeper.apache.org/doc/current/zookeeperAdmin.html)).

```
Usage of zookeeper-exporter:
  -listen string
        address to listen on (default "0.0.0.0:8080")
  -location string
        metrics location (default "/metrics")
  -timeout int
        timeout for connection to zk servers, in seconds (default 120)
  -zk-host string
        zookeeper host (default "127.0.0.1")
  -zk-list string
        comma separated list of zk servers, i.e. '10.0.0.1:2181,10.0.0.2:2181,10.0.0.3:2181', this flag overrides --zk-host/port
  -zk-port string
        zookeeper port (default "2181")
```

An example `docker-compose.yml` can be used for management of clustered zookeeper + exporters:
```
# start zk cluster and exporters
docker-compose up -d

# get metrics of first exporter (second and third exporters are on 8082 and 8083 ports)
curl -s localhost:8081/metrics

# at 8084 port there's exporter which handles multiple zk hosts
curl -s localhost:8084/metrics

# shutdown containers
docker-compose down -v
```

#### Dashboard
Example grafana dashboard: https://grafana.com/grafana/dashboards/11442
