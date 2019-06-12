// Copyright 2017, 2018 Percona LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	stdlog "log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	stdlog.SetFlags(0)
	stdlog.SetPrefix("stdlog: ")

	var (
		upstreamF         = kingpin.Flag("upstream", "Upstream exporter metrics endpoint").Default("http://127.0.0.1:9100/metrics").String()
		instanceTemplateF = kingpin.Flag("instance-template", "Instance label value template").Default("multi%d").String()
		instancesF        = kingpin.Flag("instances", "Number of instances to generate").Default("100").Int()
		logLevelF         = kingpin.Flag("log.level", "Log level").Default("info").String()
		listenF           = kingpin.Flag("web.listen-address", "Address on which to expose metrics").Default("127.0.0.1:9099").String()
	)
	kingpin.CommandLine.HelpFlag.Short('h')
	kingpin.Parse()

	level, err := logrus.ParseLevel(*logLevelF)
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.SetLevel(level)

	exporter := newExporter()
	prometheus.MustRegister(exporter)
	http.Handle("/metrics/self", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
		ErrorLog:      log.NewErrorLogger(),
		ErrorHandling: promhttp.ContinueOnError,
	}))

	client := newClient(*upstreamF)
	faker := newFaker(*instanceTemplateF, *instancesF)
	http.HandleFunc("/metrics", func(rw http.ResponseWriter, req *http.Request) {
		exporter.scrapesTotal.Inc()

		r, err := client.get()
		if err != nil {
			logrus.Error(err)
			http.Error(rw, err.Error(), 500)
			return
		}

		if err = faker.generate(rw, r); err != nil {
			logrus.Error(err)
		}
	})

	logrus.Infof("Serving fake metrics on http://%s/metrics", *listenF)
	logrus.Infof("Serving self metrics on http://%s/metrics/self", *listenF)
	logrus.Fatal(http.ListenAndServe(*listenF, nil))
}
