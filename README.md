### Prometheus zookeeper exporter

Exports `mntr` zookeeper's stats in prometheus format.
`zk_followers`, `zk_synced_followers` and `zk_pending_syncs` metrics are available only on cluster leader.

#### Build

`./build.sh` script builds `dabealu/zookeeper-exporter:latest` docker image.
To build image with different name, pass it to `build.sh` as a first arg.

#### Usage

**Note:** starting from zookeeper v3.4.10 it's required to have `mntr` command whitelisted (details: [4lw.commands.whitelist](https://zookeeper.apache.org/doc/current/zookeeperAdmin.html)).

**Warning:** flag to specify target zk hosts is changed since `v0.1.10`, see below

```
Usage of zookeeper-exporter:
  -listen string
        address to listen on (default "0.0.0.0:9141")
  -location string
        metrics location (default "/metrics")
  -timeout int
        timeout for connection to zk servers, in seconds (default 30)
  -zk-hosts string
        comma separated list of zk servers, e.g. '10.0.0.1:2181,10.0.0.2:2181,10.0.0.3:2181'
  -zk-tls-auth bool
        zk tls client authentication (default false)
  -zk-tls-auth-cert string
        tls certiticate for zk tls client authentication (required if -zk-tls-auth is true)
  -zk-tls-auth-key string
        tls key for zk tls client authentication (required if -zk-tls-auth is true)
```

An example `docker-compose.yml` can be used for management of clustered zookeeper + exporters:

```
# start zk cluster and exporters
docker-compose up -d

# get metrics of first exporter (second and third exporters are on 9142 and 9143 ports)
curl -s localhost:9141/metrics

# at 9184 port there's exporter which handles multiple zk hosts
curl -s localhost:9144/metrics

# shutdown containers
docker-compose down -v
```

#### Dashboard

Example grafana dashboard: https://grafana.com/grafana/dashboards/11442
