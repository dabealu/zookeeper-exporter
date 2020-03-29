package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
	"regexp"
)

type zkHost struct {
	Unresolved string
	*net.TCPAddr
}

type zkOptions struct {
	Timeout int64
	Hosts   []zkHost
}

// open tcp connections to zk nodes, send 'mntr' and return result as a map
func getMetrics(zkOpts zkOptions) map[string]string {
	metrics := map[string]string{}

	for _, h := range zkOpts.Hosts {
		hostLabel := fmt.Sprintf("zk_host=%q", h.Unresolved)

		// open connection
		timeout := time.Duration(zkOpts.Timeout) * time.Second
		d := net.Dialer{Timeout: timeout}
		conn, err := d.Dial("tcp", h.TCPAddr.String())
		if err != nil {
			log.Printf("warning: cannot connect to %s: %v", h.Unresolved, err)
			metrics[fmt.Sprintf("zk_up{%s}", hostLabel)] = "0"
			continue
		}

		defer conn.Close()

		_, err = conn.Write([]byte("mntr"))
		if err != nil {
			log.Printf("warning: failed to send 'mntr' to '%s': %s", h.Unresolved, err)
		}

		// read response
		res, err := ioutil.ReadAll(conn)
		if err != nil {
			log.Printf("warning: failed read 'mntr' response from '%s': %s", h.Unresolved, err)
		}

		// get slice of strings from response, like 'zk_avg_latency 0'
		lines := strings.Split(string(res), "\n")

		// skip instance if it in a leader only state and doesnt serving client requets
		if lines[0] == "This ZooKeeper instance is not currently serving requests" {
			metrics[fmt.Sprintf("zk_up{%s}", hostLabel)] = "1"
			metrics[fmt.Sprintf("zk_server_leader{%s}", hostLabel)] = "1"
			continue
		}

		// 'mntr' command isn't allowed in zk config, log as a warning
		if lines[0] == "mntr is not executed because it is not in the whitelist." {
			metrics[fmt.Sprintf("zk_up{%s}", hostLabel)] = "0"
			log.Printf("warning: mntr command isn't allowed at %s, see '4lw.commands.whitelist' ZK config parameter", hostLabel)
			continue
		}

		// split each line into key-value pair
		for _, l := range lines {
			l = strings.Replace(l, "\t", " ", -1)
			kv := strings.Split(l, " ")

			switch kv[0] {
			case "zk_server_state":
				zkLeader := fmt.Sprintf("zk_server_leader{%s}", hostLabel)
				if kv[1] == "leader" {
					metrics[zkLeader] = "1"
				} else {
					metrics[zkLeader] = "0"
				}
			
			case "zk_version":
				re := regexp.MustCompile(`^([0-9]+\.[0-9]+\.[0-9]+).*$`)
				version := re.ReplaceAllString(kv[1], "$1")

				metrics[fmt.Sprintf("zk_version{%s,version=%q}", hostLabel, version)] = "1"

			case "zk_peer_state":
				metrics[fmt.Sprintf("zk_peer_state{%s,state=%q}", hostLabel, kv[1])] = "1"
			
			case "": // noop on empty string

			default:
				metrics[fmt.Sprintf("%s{%s}", kv[0], hostLabel)] = kv[1]
			}
		}

		metrics[fmt.Sprintf("zk_up{%s}", hostLabel)] = "1"
	}

	return metrics
}

// serve zk metrics at chosen address and url
func serveMetrics(location, listen string, zkOpts zkOptions) {
	h := func(w http.ResponseWriter, r *http.Request) {
		for k, v := range getMetrics(zkOpts) {
			fmt.Fprintf(w, "%s %s\n", k, v)
		}
	}

	http.HandleFunc(location, h)
	log.Printf("info: serving metrics at %s%s", listen, location)
	if err := http.ListenAndServe(listen, nil); err != nil {
		log.Fatalf("fatal: shutting down exporter: %s", err)
	}
}

func main() {
	location := flag.String("location", "/metrics", "metrics location")
	listen := flag.String("listen", "0.0.0.0:8080", "address to listen on")
	timeout := flag.Int64("timeout", 120, "timeout for connection to zk servers, in seconds")
	host := flag.String("zk-host", "127.0.0.1", "zookeeper host")
	port := flag.String("zk-port", "2181", "zookeeper port")
	list := flag.String("zk-list", "",
		"comma separated list of zk servers, i.e. '10.0.0.1:2181,10.0.0.2:2181,10.0.0.3:2181', this flag overrides --zk-host/port")
	flag.Parse()

	Hosts := []zkHost{}

	if *list == "" {
		// single zk server
		h := *host + ":" + *port
		tcp, err := net.ResolveTCPAddr("tcp", h)
		if err != nil {
			log.Fatalf("fatal: cannot resolve zk hostname '%s': %s", h, err)
		}

		Hosts = append(Hosts, zkHost{Unresolved: h, TCPAddr: tcp})
	} else {
		// list of zk servers, 
		for _, h := range strings.Split(*list, ",") {
			tcp, err := net.ResolveTCPAddr("tcp", h)
			if err != nil {
				log.Printf("warning: cannot resolve zk hostname '%s': %s", h, err)
				continue
			}

			Hosts = append(Hosts, zkHost{Unresolved: h, TCPAddr: tcp})
		}
	}

	log.Printf("info: zookeeper addresses %v", Hosts)

	zkOpts := zkOptions{
		Timeout: *timeout,
		Hosts:   Hosts,
	}
	serveMetrics(*location, *listen, zkOpts)
}
