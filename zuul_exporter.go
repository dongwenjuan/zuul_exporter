// Copyright 2013 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
    "fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
    "sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "zuul" // For Prometheus metrics.
)

func init() {
	prometheus.MustRegister(version.NewCollector("zuul_exporter"))
}

type zuulHost struct {
	hostname    string
	port        string
}

type Exporter struct {
	scrapeHosts     []zuulHost
    mutex           sync.Mutex
	client          *http.Client
	up              *prometheus.Desc
	scrapeFailures  prometheus.Counter
	zuulVersion     *prometheus.Desc
}

func NewExporter(zuulhosts []zuulHost) *Exporter {
	return &Exporter{
		scrapeHosts: zuulhosts,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: nil},
		},
		up: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "up"),
			"Could the zuul server be reached",
			[]string{"host"},
			nil),
		scrapeFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrape_failures_total",
			Help:      "Number of errors while scraping apache.",
		}),
		zuulVersion: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "", "version"),
			"The version of zuul server",
			nil,
			nil),
	}
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.up
	e.scrapeFailures.Describe(ch)
	ch <- e.zuulVersion
}

func (e *Exporter) collect(ch chan<- prometheus.Metric) error {
	for _, host := range e.scrapeHosts {
		scrapeURI := "http://" + host.hostname + ":" + host.port + "/status"
		resp, err := e.client.Get(scrapeURI)
		if err != nil {
			ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 0, host.hostname)
			return fmt.Errorf("Error scraping zuul: %v", err)
		}
		ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 1, host.hostname)

//		data, err := ioutil.ReadAll(resp.Body)
//		resp.Body.Close()
//		if resp.StatusCode != 200 {
//			if err != nil {
//				data = []byte(err.Error())
//			}
//			return fmt.Errorf("Status %s (%d): %s", resp.Status, resp.StatusCode, data)
//		}
//	}
    return nil
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()
	if err := e.collect(ch); err != nil {
		log.Errorf("Error scraping zuul: %s", err)
		e.scrapeFailures.Inc()
		e.scrapeFailures.Collect(ch)
	}
	return
}

func main() {
	var (
		listenAddress   = kingpin.Flag("web.listen-address", "The address on which to expose the web interface and generated Prometheus metrics.").Default(":9532").String()
		metricsEndpoint = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		zuulAddressList = kingpin.Flag("zuul.listen-address-list", "The zuul list addresses.").Default("").String()
	)

	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("zuul_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	zuulhosts := []zuulHost{}
	if *zuulAddressList == "" {
		log.Fatalln("zuul.listen-address-list must be specified for collect metrics.")
	}
	for _, address := range strings.Split(*zuulAddressList, ",") {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			log.Fatalln("Bad one zuul listen address: %s .", address)
		}
		zuulhosts = append(zuulhosts, zuulHost{hostname: host, port: port})
	}

	exporter := NewExporter(zuulhosts)
	prometheus.MustRegister(exporter)

	log.Infoln("Starting Zuul -> Prometheus Exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())
	log.Infof("Accepting zuul address: %s", *zuulAddressList)
	log.Infof("Accepting Prometheus Requests on %s", *listenAddress)

	http.Handle(*metricsEndpoint, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Zuul Exporter</title></head>
			<body>
			<h1>Zuul Exporter</h1>
			<p><a href="` + *metricsEndpoint + `">Metrics</a></p>
			</body>
			</html>`))
	})
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
