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

func (s *Service) Run() {
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

func (s *Service) handle(ctx context.Context, tc TestCase) {
	start := time.Now()
	o := &O{
		w:     s.w,
		stats: s.stats,
	}
	to := s.defaultTimeout
	if tc.Timeout > 10*time.Millisecond {
		to = tc.Timeout
	}
	c, cancel := context.WithTimeout(ctx, to)
	defer cancel()
	tc.Func(c, o)
	if c.Err() != nil && !o.failed {
		o.Errorf("failed on context error: %v", c.Err())
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
}

func (s *Service) run(tc TestCase) {
	tick := time.NewTicker(tc.Period)
	// Waitgroup for different invocations of this test case
	var wg sync.WaitGroup

loop:
	for {
		select {
		case <-tick.C:
		case <-s.done:
			tick.Stop()
			break loop
		}
		ctx, cancel := context.WithCancel(context.Background())
		complete := make(chan struct{})
		wg.Add(1)
		go func(c context.Context) {
			s.handle(c, tc)
			close(complete)
			wg.Done()
		}(ctx)
		// Cancel the above goroutine on shutdown without blocking the loop
		go func(comp chan struct{}, c context.CancelFunc) {
			select {
			case <-s.done:
				// TODO: make this configurable at the service level.  i.e.
				// provide the option to allow the inflight tests to finish
				// without failing
				c()
			case <-comp:
			}
		}(complete, cancel)
	}
	wg.Wait()
	s.wg.Done()
}

func (s *Service) Close() error {
	s.once.Do(func() {
		close(s.done)
	})
	s.wg.Wait()
	return nil
}
