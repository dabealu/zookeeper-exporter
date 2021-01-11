package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
	"os"
)

func main() {
	location := flag.String("location", "/metrics", "metrics location")
	listen := flag.String("listen", "0.0.0.0:9141", "address to listen on")
	timeout := flag.Int64("timeout", 30, "timeout for connection to zk servers, in seconds")
	zkhosts := flag.String("zk-hosts", "", "comma separated list of zk servers, e.g. '10.0.0.1:2181,10.0.0.2:2181,10.0.0.3:2181'")

	flag.Parse()

	hosts := strings.Split(*zkhosts, ",")
	if len(hosts) == 0 || (len(hosts) == 1 && hosts[0] == "") {
		log.Print("fatal: no target zookeeper hosts specified.")
		flag.Usage()
		os.Exit(1)
	}

	log.Printf("info: zookeeper hosts: %v", hosts)

	options := Options{
		Timeout:  *timeout,
		Hosts:    hosts,
		Location: *location,
		Listen:   *listen,
	}

	log.Printf("info: serving metrics at %s%s", *listen, *location)
	serveMetrics(&options)
}

type Options struct {
	Timeout  int64
	Hosts    []string
	Location string
	Listen   string
}

const cmdNotExecutedSffx = "is not executed because it is not in the whitelist."

var versionRE = regexp.MustCompile(`^([0-9]+\.[0-9]+\.[0-9]+).*$`)

var metricNameReplacer = strings.NewReplacer("-", "_", ".", "_")

// open tcp connections to zk nodes, send 'mntr' and return result as a map
func getMetrics(options *Options) map[string]string {
	metrics := map[string]string{}
	timeout := time.Duration(options.Timeout) * time.Second
	dialer := net.Dialer{Timeout: timeout}

	for _, h := range options.Hosts {
		tcpaddr, err := net.ResolveTCPAddr("tcp", h)
		if err != nil {
			log.Printf("warning: cannot resolve zk hostname '%s': %s", h, err)
			continue
		}

		hostLabel := fmt.Sprintf("zk_host=%q", h)
		zkUp := fmt.Sprintf("zk_up{%s}", hostLabel)

		conn, err := dialer.Dial("tcp", tcpaddr.String())
		if err != nil {
			log.Printf("warning: cannot connect to %s: %v", h, err)
			metrics[zkUp] = "0"
			continue
		}

		res := sendZookeeperCmd(conn, h, "mntr")

		// get slice of strings from response, like 'zk_avg_latency 0'
		lines := strings.Split(res, "\n")

		// skip instance if it in a leader only state and doesnt serving client requets
		if lines[0] == "This ZooKeeper instance is not currently serving requests" {
			metrics[zkUp] = "1"
			metrics[fmt.Sprintf("zk_server_leader{%s}", hostLabel)] = "1"
			continue
		}

		// 'mntr' command isn't allowed in zk config, log as a warning
		if strings.Contains(lines[0], cmdNotExecutedSffx) {
			metrics[zkUp] = "0"
			logNotAllowed("mntr", hostLabel)
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
				version := versionRE.ReplaceAllString(kv[1], "$1")

				metrics[fmt.Sprintf("zk_version{%s,version=%q}", hostLabel, version)] = "1"

			case "zk_peer_state":
				metrics[fmt.Sprintf("zk_peer_state{%s,state=%q}", hostLabel, kv[1])] = "1"

			case "": // noop on empty string

			default:
				metrics[fmt.Sprintf("%s{%s}", metricNameReplacer.Replace(kv[0]), hostLabel)] = kv[1]
			}
		}

		zkRuok := fmt.Sprintf("zk_ruok{%s}", hostLabel)
		if conn, err = dialer.Dial("tcp", tcpaddr.String()); err == nil {
			res = sendZookeeperCmd(conn, h, "ruok")
			if res == "imok" {
				metrics[zkRuok] = "1"
			} else {
				if strings.Contains(res, cmdNotExecutedSffx) {
					logNotAllowed("ruok", hostLabel)
				}
				metrics[zkRuok] = "0"
			}
		} else {
			metrics[zkRuok] = "0"
		}

		metrics[zkUp] = "1"
	}

	return metrics
}

func logNotAllowed(cmd, label string) {
	log.Printf("warning: %s command isn't allowed at %s, see '4lw.commands.whitelist' ZK config parameter", cmd, label)
}

func sendZookeeperCmd(conn net.Conn, host, cmd string) string {
	defer conn.Close()

	_, err := conn.Write([]byte(cmd))
	if err != nil {
		log.Printf("warning: failed to send '%s' to '%s': %s", cmd, host, err)
	}

	res, err := ioutil.ReadAll(conn)
	if err != nil {
		log.Printf("warning: failed read '%s' response from '%s': %s", cmd, host, err)
	}

	return string(res)
}

// serve zk metrics at chosen address and url
func serveMetrics(options *Options) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		for k, v := range getMetrics(options) {
			fmt.Fprintf(w, "%s %s\n", k, v)
		}
	}

	http.HandleFunc(options.Location, handler)

	if err := http.ListenAndServe(options.Listen, nil); err != nil {
		log.Fatalf("fatal: shutting down exporter: %s", err)
	}
}
