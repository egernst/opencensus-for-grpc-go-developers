// Copyright 2018, OpenCensus Authors
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
	"bytes"
	"context"
	"log"
	"net"
	"net/http"

	"google.golang.org/grpc"

	"contrib.go.opencensus.io/exporter/prometheus"
	"contrib.go.opencensus.io/exporter/zipkin"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"

	openzipkin "github.com/openzipkin/zipkin-go"
	zipkinHTTP "github.com/openzipkin/zipkin-go/reporter/http"
	"github.com/orijtech/opencensus-for-grpc-go-developers/rpc"
)

type fetchIt int

// Compile time assertion that fetchIt implements FetchServer.
var _ rpc.FetchServer = (*fetchIt)(nil)

func (fi *fetchIt) Capitalize(ctx context.Context, in *rpc.Payload) (*rpc.Payload, error) {
	_, span := trace.StartSpan(ctx, "(*fetchIt).Capitalize")
	defer span.End()

	span.Annotate([]trace.Attribute{
		trace.Int64Attribute("len", int64(len(in.Data))),
	}, "Data in")
	out := &rpc.Payload{
		Data: bytes.ToUpper(in.Data),
	}
	span.Annotate([]trace.Attribute{trace.StringAttribute("name", "eric")}, "spaghetti")
	return out, nil
}

func main() {
	addr := ":9988"
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("gRPC server: failed to listen: %v", err)
	}
	if err := view.Register(ocgrpc.DefaultServerViews...); err != nil {
		log.Fatalf("Failed to register gRPC server views: %v", err)
	}
	srv := grpc.NewServer(grpc.StatsHandler(new(ocgrpc.ServerHandler)))
	rpc.RegisterFetchServer(srv, new(fetchIt))
	log.Printf("fetchIt gRPC server serving at %q", addr)

	// OpenCensus exporters
	createAndRegisterExporters()

	// Finally serve
	if err := srv.Serve(ln); err != nil {
		log.Fatalf("gRPC server: error serving: %v", err)
	}
}

func createAndRegisterExporters() {
	// For demo purposes, set this to always sample.
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	// 1. Prometheus
	prefix := "fetchit"
	pe, err := prometheus.NewExporter(prometheus.Options{
		Namespace: prefix,
	})
	if err != nil {
		log.Fatalf("Failed to create Prometheus exporter: %v", err)
	}
	view.RegisterExporter(pe)
	// We need to expose the Prometheus collector via an endpoint /metrics
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", pe)
		log.Fatal(http.ListenAndServe(":9888", mux))
	}()

	// 2. Register Zipkin
	localEndpoint, err := openzipkin.NewEndpoint("mygRPCServerZipkin", "192.168.1.61:8080")
	if err != nil {
		log.Fatalf("Failed to create Zipkin exporter: %v", err)
	}
	reporter := zipkinHTTP.NewReporter("http://localhost:9411/api/v2/spans")
	exporter := zipkin.NewExporter(reporter, localEndpoint)
	trace.RegisterExporter(exporter)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

}
