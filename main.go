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

// open tcp connection to zk node, send 'mntr' and return result as a map
func getMetrics(address *net.TCPAddr) map[string]string {
	// open connection
	conn, err := net.DialTCP("tcp", nil, address)
	if err != nil {
		log.Printf("warning: cannot connect to %v", address)
		return map[string]string{"zk_up": "0"}
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
	metrics := map[string]string{}
	for _, l := range lines {
		l = strings.Replace(l, "\t", " ", -1)
		kv := strings.Split(l, " ")

		if kv[0] == "zk_server_state" {
			if kv[1] == "leader" {
				metrics["zk_server_leader"] = "1"
			} else {
				metrics["zk_server_leader"] = "0"
			}
		} else if kv[0] == "zk_version" {
			metrics[kv[0]] = strings.Join(kv[1:], " ")
		} else if kv[0] != "" {
			metrics[kv[0]] = kv[1]
		}
	}
	metrics["zk_up"] = "1"

	return metrics
}

func serveMetrics(location, listen string, zk *net.TCPAddr) {
	h := func(w http.ResponseWriter, r *http.Request) {
		metrics := getMetrics(zk)
		for k, v := range metrics {
			if k == "zk_version" {
				fmt.Fprintf(w, "zk_version{version=\"%s\"} 1\n", v)
			} else {
				fmt.Fprintf(w, "%s %s\n", k, v)
			}
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
	zkHost := flag.String("zk-host", "127.0.0.1", "zookeeper host")
	zkPort := flag.String("zk-port", "2181", "zookeeper port")
	flag.Parse()

	// get *net.TCPAddr
	zkTCP, err := net.ResolveTCPAddr("tcp", *zkHost+":"+*zkPort)
	errFatal(err)

	log.Printf("zookeeper address %v", zkTCP)
	serveMetrics(*location, *listen, zkTCP)
}
