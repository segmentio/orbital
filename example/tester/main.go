package main

import (
	"context"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/justinas/alice"
	"github.com/segmentio/conf"
	"github.com/segmentio/orbital/example"
	"github.com/segmentio/orbital/orbital"
	"github.com/segmentio/orbital/webhook"
	"github.com/tidwall/gjson"

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
		Address: ":3001",
	}
	prog string = path.Base(os.Args[0])
)

func main() {
	events.Log("%{program}s version: %{version}s", prog, version)
	conf.Load(&config)
	events.Log("service starting with config: %+{config}v", config)
	initLogMetrics()

	// Configure end-to-end tests
	orb := orbital.New(
		orbital.WithStats(stats.DefaultEngine),
		orbital.WithTimeout(5*time.Second),
	)
	// Manages hooking events back into the test.  The tests must know to wait
	// on this ID
	rl := webhook.NewRouteLogger(func(r webhook.Request) string {
		return gjson.Get(r.Body, "id").Str
	})
	// receives the events and logs them
	wh := webhook.New(webhook.Config{
		Logger: rl,
	})
	// Configuration for the test.
	eh := example.Harness{
		API:    "http://localhost:3000/",
		Waiter: rl,
	}
	orb.Register(orbital.TestCase{
		Name:    "smoke test",
		Period:  1 * time.Second,
		Timeout: 3 * time.Second,
		Func:    eh.OrbitalSmoke,
	})
	orb.Run()

	mux := http.NewServeMux()
	chain := alice.New(httpstats.NewHandler, httpevents.NewHandler)
	mux.HandleFunc("/internal/health", health)
	mux.Handle("/internal/metrics", prometheus.DefaultHandler)
	mux.Handle("/", chain.Then(wh))

	server := &http.Server{Addr: config.Address, Handler: mux}
	errc := make(chan error)
	go func() {
		errc <- server.ListenAndServe()
	}()

	sigchan := make(chan os.Signal)
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
	orb.Close()
}

// health implements an ELB health check and responds with an HTTP 200.
func health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(200)
}

type cfg struct {
	Address string `conf:"address" help:"address on which the server should listen"`
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
