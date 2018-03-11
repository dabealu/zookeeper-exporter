package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
)

func errFatal(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

type zkHost struct {
	Unresolved string
	*net.TCPAddr
}

// open tcp connections to zk nodes, send 'mntr' and return result as a map
func getMetrics(hosts []zkHost) map[string]string {
	metrics := map[string]string{}

	for _, h := range hosts {
		hostLabel := fmt.Sprintf("zk_host=%q", h.Unresolved)

		// open connection
		conn, err := net.DialTCP("tcp", nil, h.TCPAddr)
		if err != nil {
			log.Printf("warning: cannot connect to %s: %v", h.Unresolved, err)
			metrics[fmt.Sprintf("zk_up{%s}", hostLabel)] = "0"
			continue
		}

		defer conn.Close()

		_, err = conn.Write([]byte("mntr"))
		errFatal(err)

		// read response
		res, err := ioutil.ReadAll(conn)
		errFatal(err)

		// get slice of strings from response, like 'zk_avg_latency 0'
		lines := strings.Split(string(res), "\n")

		// split each line into key-value pair
		for _, l := range lines {
			l = strings.Replace(l, "\t", " ", -1)
			kv := strings.Split(l, " ")

			if kv[0] == "zk_server_state" {
				zkLeader := fmt.Sprintf("zk_server_leader{%s}", hostLabel)
				if kv[1] == "leader" {
					metrics[zkLeader] = "1"
				} else {
					metrics[zkLeader] = "0"
				}
			} else if kv[0] == "zk_version" {
				zkVersion := fmt.Sprintf("zk_version{%s,version=%q}", hostLabel, strings.Join(kv[1:], " "))
				metrics[zkVersion] = "1"
			} else if kv[0] != "" {
				metrics[fmt.Sprintf("%s{%s}", kv[0], hostLabel)] = kv[1]
			}
		}

		metrics[fmt.Sprintf("zk_up{%s}", hostLabel)] = "1"
	}

	return metrics
}

// serve zk metrics at chosen address and url
func serveMetrics(location, listen string, zk []zkHost) {
	h := func(w http.ResponseWriter, r *http.Request) {
		for k, v := range getMetrics(zk) {
			fmt.Fprintf(w, "%s %s\n", k, v)
		}
	}

	http.HandleFunc(location, h)
	log.Printf("starting serving metrics at %s%s", listen, location)
	err := http.ListenAndServe(listen, nil)
	errFatal(err)
}

func main() {
	location := flag.String("location", "/metrics", "metrics location")
	listen := flag.String("listen", "0.0.0.0:8080", "address to listen on")
	host := flag.String("zk-host", "127.0.0.1", "zookeeper host")
	port := flag.String("zk-port", "2181", "zookeeper port")
	list := flag.String("zk-list", "",
		"comma separated list of zk servers, i.e. '10.0.0.1:2181,10.0.0.2:2181,10.0.0.3:2181', this flag overrides --zk-host/port")
	flag.Parse()

	zkHosts := []zkHost{}

	if *list == "" {
		h := *host + ":" + *port
		tcp, err := net.ResolveTCPAddr("tcp", h)
		errFatal(err)
		zkHosts = append(zkHosts, zkHost{Unresolved: h, TCPAddr: tcp})
	} else {
		for _, h := range strings.Split(*list, ",") {
			tcp, err := net.ResolveTCPAddr("tcp", h)
			errFatal(err)
			zkHosts = append(zkHosts, zkHost{Unresolved: h, TCPAddr: tcp})
		}
	}

	log.Printf("zookeeper addresses %v", zkHosts)
	serveMetrics(*location, *listen, zkHosts)
}
