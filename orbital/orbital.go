package orbital

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/segmentio/stats"
)

// TestCase represents an individual test to be run on a schedule given by
// Period.  If Timeout is not specified, Service will provide a default timeout.
// Name should be metrics-compatible, for now.
type TestCase struct {
	Period  time.Duration
	Name    string
	Func    TestFunc
	Timeout time.Duration
	Tags    []stats.Tag
}

// TestFunc represents a function to be run under test
// Should block until complete
// Responsible for cleaning up any allocated resources
// or spawned goroutines before returning
// Should select on ctx.Done for long running operations
type TestFunc func(ctx context.Context, o *O)

// O is the base construct for orbital.  It should be used for logging,
// metrics access and most importantly, signaling if a test has failed.
type O struct {
	// output writer
	w io.Writer

	stats  *stats.Engine
	failed bool
	mu     sync.Mutex
}

// Error is equivalent to Log followed by Fail
func (o *O) Error(args ...interface{}) {
	o.log(fmt.Sprintln(args...))
	o.Fail()
}

// Errorf is equivalent to Logf followed by Fail
func (o *O) Errorf(fstr string, args ...interface{}) {
	o.log(fmt.Sprintf(fstr, args...))
	o.Fail()
}

// Fatal functions exist in testing.T because they call
// runtime.Goexit to prevent having to deal with context
// and because it's convenient.
// Not sure they're worth having in an always running
// daemon
// func (o *O) Fatal(args ...interface{}) {
// 	o.log(fmt.Sprintln(args...))
// }
//
// func (o *O) Fatalf(fstr string, args ...interface{}) {
// 	o.log(fmt.Sprintf(fstr, args...))
// }

func (o *O) Log(args ...interface{}) {
	o.log(fmt.Sprintln(args...))
}

func (o *O) Logf(fstr string, args ...interface{}) {
	o.log(fmt.Sprintf(fstr, args...))
}

func (o *O) log(s string) {
	if len(s) == 0 || s[len(s)-1] != '\n' {
		s += "\n"
	}

	fmt.Fprint(o.w, s)
}

func (o *O) Fail() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.failed = true
}

func (o *O) Stats() *stats.Engine {
	return o.stats
}
