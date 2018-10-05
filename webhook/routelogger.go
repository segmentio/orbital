package webhook

import (
	"context"
	"reflect"
	"sync"

	"github.com/pkg/errors"
	"github.com/segmentio/events"
)

// RouteLogger is a struct to enable being notified when an request identified
// by a key has been received by the   It satisfies the Logger
// interface
type RouteLogger struct {
	key func(Request) string

	rc map[string]chan Request
	mu sync.Mutex
}

// NewRouteLogger takes a key function which can generate a string key based
// upon the given Request.  This key will be used to notify goroutines blocked
// on RouteLogger.Wait.
func NewRouteLogger(key func(Request) string) *RouteLogger {
	return &RouteLogger{
		rc:  make(map[string]chan Request),
		key: key,
	}
}

func (s *RouteLogger) Record(r Request) {
	events.Debug("%+v", r)
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(r)
	c, ok := s.rc[k]

	if !ok {
		return
	}

	// Requests are dropped if writing chan would block
	select {
	case c <- r:
	default:
		events.Debug("chan full, dropping request")
	}
}

// Sent prepares RouteLogger to be able to handle the incoming request Sent
// returns an error if the key already exists in the map. Keys must be unique
func (s *RouteLogger) Sent(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.rc[key]; ok {
		return errors.Errorf("key %s already sent", key)
	}
	// Channel buffer 1 prevents the race between receiving an event on the
	// webhook and calling `Wait`.  Otherwise, the event would be dropped in
	// the select statement for Record
	s.rc[key] = make(chan Request, 1)
	return nil
}

// Wait blocks until it's received a message on the webhook matching the key,
// or ctx.Done() is triggered.  If Wait is called before Sent, an error is
// returned.
func (s *RouteLogger) Wait(ctx context.Context, key string) (Request, error) {
	s.mu.Lock()
	c, ok := s.rc[key]
	if !ok {
		return Request{}, errors.New("Wait called before Sent")
	}
	s.mu.Unlock()
	select {
	case r := <-c:
		return r, nil
	case <-ctx.Done():
		return Request{}, ctx.Err()
	}
}

// WaitN waits for n events within the given keys.
// If WaitFor is called before one the the given keys has been sent, an error
// is returned.
func (s *RouteLogger) WaitN(ctx context.Context, n int, keys ...string) ([]Request, error) {
	requests := make([]Request, 0, len(keys))

	cases := []reflect.SelectCase{
		{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ctx.Done()),
		},
	}

	s.mu.Lock()
	for _, key := range keys {
		c, ok := s.rc[key]
		if !ok {
			return nil, errors.Errorf("WaitAll called before key %s was sent", key)
		}

		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(c),
		})
	}
	s.mu.Unlock()

	for n > 0 {
		chosen, value, ok := reflect.Select(cases)

		switch chosen {
		case 0:
			// ctx.Done().
			return nil, ctx.Err()

		default:
			// The chan is closed so we stop to listen on this channel.
			if !ok {
				cases[chosen].Chan = reflect.ValueOf(nil)
				continue
			}

			req := Request{}
			if req, ok = value.Interface().(Request); !ok {
				return nil, errors.Errorf("WaitAll did not get a Request: %T", value.Type())
			}

			requests = append(requests, req)
			n--
		}
	}
	return requests, nil
}

// Delete must be called after any successful call to Sent to free allocated
// resources. Not doing so results in a leak.
func (s *RouteLogger) Delete(key string) {
	s.mu.Lock()
	delete(s.rc, key)
	s.mu.Unlock()
}
