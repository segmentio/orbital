package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/justinas/alice"
	"github.com/segmentio/conf"
	"github.com/tidwall/sjson"

	"github.com/segmentio/events"
	_ "github.com/segmentio/events/ecslogs"
	"github.com/segmentio/events/httpevents"
	_ "github.com/segmentio/events/log"
	_ "github.com/segmentio/events/sigevents"
	_ "github.com/segmentio/events/text"

	"github.com/segmentio/stats"
	"github.com/segmentio/stats/httpstats"
	"github.com/segmentio/stats/prometheus"
)

var (
	version = "dev"
	config  = cfg{
		Address:  ":3000",
		Upstream: "localhost:3001",
	}
	prog string = path.Base(os.Args[0])
)

func main() {
	events.Log("%{program}s version: %{version}s", prog, version)
	conf.Load(&config)
	events.Log("service starting with config: %+{config}v", config)
	initLogMetrics()

	// Copied from defaultDirector in httputil, with modification
	forward := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = config.Upstream

			bs, err := ioutil.ReadAll(req.Body)
			if err != nil {
				panic(err)
			}
			ss, err := sjson.Set(string(bs), "processed", true)
			if err != nil {
				panic(err)
			}
			req.ContentLength = int64(len(ss))
			req.Body = ioutil.NopCloser(bytes.NewBufferString(ss))
		},
	}

	mux := http.NewServeMux()
	chain := alice.New(httpstats.NewHandler, httpevents.NewHandler)
	mux.HandleFunc("/internal/health", health)
	mux.Handle("/", chain.Then(forward))

	server := &http.Server{Addr: config.Address, Handler: mux}
	errc := make(chan error)
	go func() {
		errc <- server.ListenAndServe()
	}()

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigchan:
		events.Log("stopping in response to signal %{signal}s.", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel() // <- govet
		server.Shutdown(ctx)
	case err := <-errc:
		events.Log("stopping in error %+{error}v.", err)
	}
}

// health implements an ELB health check and responds with an HTTP 200.
func health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(200)
}

type cfg struct {
	Address  string `conf:"address" help:"address on which the server should listen"`
	Upstream string `conf:"upstream" help:"proxy upstream addr"`
}

func initLogMetrics() {
	events.DefaultLogger.Args = events.Args{events.Arg{Name: "version", Value: version}}
	events.DefaultLogger.EnableDebug = false
	stats.DefaultEngine = stats.NewEngine(prog, prometheus.DefaultHandler,
		stats.T("version", version),
	)
	// Force a metrics flush every second
	go func() {
		for range time.Tick(time.Second) {
			stats.Flush()
		}
	}()
}
