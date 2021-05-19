package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// template format: command, host_label
	commandNotAllowedTmpl     = "warning: %q command isn't allowed at %q, see '4lw.commands.whitelist' ZK config parameter"
	instanceNotServingMessage = "This ZooKeeper instance is not currently serving requests"
	cmdNotExecutedSffx        = "is not executed because it is not in the whitelist."
)

var (
	versionRE          = regexp.MustCompile(`^([0-9]+\.[0-9]+\.[0-9]+).*$`)
	metricNameReplacer = strings.NewReplacer("-", "_", ".", "_")
)

func main() {
	location := flag.String("location", "/metrics", "metrics location")
	listen := flag.String("listen", "0.0.0.0:9141", "address to listen on")
	timeout := flag.Int64("timeout", 30, "timeout for connection to zk servers, in seconds")
	zkhosts := flag.String("zk-hosts", "", "comma separated list of zk servers, e.g. '10.0.0.1:2181,10.0.0.2:2181,10.0.0.3:2181'")
	zktlsauth := flag.Bool("zk-tls-auth", false, "zk tls client authentication")
	zktlscert := flag.String("zk-tls-auth-cert", "", "cert for zk tls client authentication")
	zktlskey := flag.String("zk-tls-auth-key", "", "key for zk tls client authentication")

	flag.Parse()

	var clientCert *tls.Certificate
	if *zktlsauth {
		if *zktlscert == "" || *zktlskey == "" {
			log.Fatal("-zk-tls-auth-cert and -zk-tls-auth-key flags are required when -zk-tls-auth is true")
		}
		_clientCert, err := tls.LoadX509KeyPair(*zktlscert, *zktlskey)
		if err != nil {
			log.Fatalf("fatal: can't load keypair %s, %s: %v", *zktlskey, *zktlscert, err)
		}
		clientCert = &_clientCert
	}

	hosts := strings.Split(*zkhosts, ",")
	if len(hosts) == 0 {
		log.Fatal("fatal: no target zookeeper hosts specified, exiting")
	}

	log.Printf("info: zookeeper hosts: %v", hosts)
	log.Printf("info: serving metrics at %s%s", *listen, *location)
	serveMetrics(&Options{
		Timeout:    *timeout,
		Hosts:      hosts,
		Location:   *location,
		Listen:     *listen,
		ClientCert: clientCert,
	})
}

type Options struct {
	Timeout    int64
	Hosts      []string
	Location   string
	Listen     string
	ClientCert *tls.Certificate
}

func dial(host string, timeout time.Duration, clientCert *tls.Certificate) (net.Conn, error) {
	dialer := net.Dialer{Timeout: timeout}
	if clientCert == nil {
		return dialer.Dial("tcp", host)
	} else {
		return tls.DialWithDialer(&dialer, "tcp", host, &tls.Config{
			Certificates:       []tls.Certificate{*clientCert},
			InsecureSkipVerify: true,
		})
	}
}

// open tcp connections to zk nodes, send 'mntr' and return result as a map
func getMetrics(options *Options) map[string]string {
	metrics := map[string]string{}
	timeout := time.Duration(options.Timeout) * time.Second

	for _, h := range options.Hosts {
		tcpaddr, err := net.ResolveTCPAddr("tcp", h)
		if err != nil {
			log.Printf("warning: cannot resolve zk hostname '%s': %s", h, err)
			continue
		}

		hostLabel := fmt.Sprintf("zk_host=%q", h)
		zkUp := fmt.Sprintf("zk_up{%s}", hostLabel)

		conn, err := dial(tcpaddr.String(), timeout, options.ClientCert)
		if err != nil {
			log.Printf("warning: cannot connect to %s: %v", h, err)
			metrics[zkUp] = "0"
			continue
		}

		res := sendZookeeperCmd(conn, h, "mntr")

		// get slice of strings from response, like 'zk_avg_latency 0'
		lines := strings.Split(res, "\n")

		// skip instance if it in a leader only state and doesnt serving client requets
		if lines[0] == instanceNotServingMessage {
			metrics[zkUp] = "1"
			metrics[fmt.Sprintf("zk_server_leader{%s}", hostLabel)] = "1"
			continue
		}

		// 'mntr' command isn't allowed in zk config, log as a warning
		if strings.Contains(lines[0], cmdNotExecutedSffx) {
			metrics[zkUp] = "0"
			log.Printf(commandNotAllowedTmpl, "mntr", hostLabel)
			continue
		}

		// split each line into key-value pair
		for _, l := range lines {
			if l == "" {
				continue
			}

			kv := strings.Split(strings.Replace(l, "\t", " ", -1), " ")
			key := kv[0]
			value := kv[1]

			switch key {
			case "zk_server_state":
				zkLeader := fmt.Sprintf("zk_server_leader{%s}", hostLabel)
				if value == "leader" {
					metrics[zkLeader] = "1"
				} else {
					metrics[zkLeader] = "0"
				}

			case "zk_version":
				version := versionRE.ReplaceAllString(value, "$1")
				metrics[fmt.Sprintf("zk_version{%s,version=%q}", hostLabel, version)] = "1"

			case "zk_peer_state":
				metrics[fmt.Sprintf("zk_peer_state{%s,state=%q}", hostLabel, value)] = "1"

			default:
				var k string
				if strings.Contains(key, "}") {
					k = metricNameReplacer.Replace(key)
					k = strings.Replace(k, "}", ",", 1)
					k = fmt.Sprintf("%s%s}", k, hostLabel)
				} else {
					k = fmt.Sprintf("%s{%s}", metricNameReplacer.Replace(key), hostLabel)
				}

				if !isDigit(value) {
					log.Printf("warning: skipping metric %q which holds not-digit value: %q", key, value)
					continue
				}

				metrics[k] = value
			}
		}

		zkRuok := fmt.Sprintf("zk_ruok{%s}", hostLabel)
		if conn, err := dial(tcpaddr.String(), timeout, options.ClientCert); err == nil {
			res = sendZookeeperCmd(conn, h, "ruok")
			if res == "imok" {
				metrics[zkRuok] = "1"
			} else {
				if strings.Contains(res, cmdNotExecutedSffx) {
					log.Printf(commandNotAllowedTmpl, "ruok", hostLabel)
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

func isDigit(in string) bool {
	// check input is an int
	if _, err := strconv.Atoi(in); err != nil {
		// not int, try float
		if _, err := strconv.ParseFloat(in, 64); err != nil {
			return false
		}
	}
	return true
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
