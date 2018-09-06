package orbital

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/segmentio/stats"
)

// Service runs all registered TestCases on the schedule specified during
// registration.
type Service struct {
	// list of tests to run
	tests   map[string]TestCase
	stats   *stats.Engine
	mu      sync.Mutex
	started bool

	defaultTimeout time.Duration

	w io.Writer

	done chan struct{}
	once sync.Once
	wg   sync.WaitGroup
}

func WithStats(s *stats.Engine) func(*Service) {
	return func(svc *Service) {
		svc.stats = s
	}
}

func WithTimeout(d time.Duration) func(*Service) {
	return func(svc *Service) {
		svc.defaultTimeout = d
	}
}

func New(opts ...func(*Service)) *Service {
	s := &Service{
		w:              os.Stderr,
		done:           make(chan struct{}),
		tests:          make(map[string]TestCase),
		defaultTimeout: 10 * time.Minute,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

var DefaultService = New()

func Register(tc TestCase) {
	DefaultService.Register(tc)
}

// Register a test case to be run.
func (s *Service) Register(tc TestCase) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tests[tc.Name] = tc
}

func (s *Service) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stats == nil {
		s.stats = stats.DefaultEngine
	}
	if !s.started {
		for _, v := range s.tests {
			s.wg.Add(1)
			go s.run(v)
		}
	}
	s.started = true
}

func (s *Service) run(tc TestCase) {
	tick := time.NewTicker(tc.Period)

loop:
	for {
		select {
		case <-tick.C:
			// TODO: consider clean shutdown for each instance of a run
			go func() {
				start := time.Now()
				o := &O{
					w:     s.w,
					stats: s.stats,
				}
				to := s.defaultTimeout
				if tc.Timeout > 1*time.Second {
					to = tc.Timeout
				}
				c, cancel := context.WithTimeout(context.Background(), to)
				defer cancel()
				tc.Func(c, o)
				if c.Err() != nil {
					o.Fail()
				}
				dur := time.Now().Sub(start)
				if o.failed {
					tags := append([]stats.Tag{
						stats.T("case", tc.Name),
						stats.T("result", "fail"),
					}, tc.Tags...)

					s.stats.Observe("case", dur, tags...)
					fmt.Fprintf(s.w, "--- FAIL: %s (%s)\n", tc.Name, dur)
				} else {
					tags := append([]stats.Tag{
						stats.T("case", tc.Name),
						stats.T("result", "pass"),
					}, tc.Tags...)

					s.stats.Observe("case", dur, tags...)
					fmt.Fprintf(s.w, "--- PASS: %s (%s)\n", tc.Name, dur)
				}
			}()
		case <-s.done:
			tick.Stop()
			break loop
		}
	}
	s.wg.Done()
}

func (s *Service) Close() error {
	s.once.Do(func() {
		close(s.done)
	})
	s.wg.Wait()
	return nil
}
