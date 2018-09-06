package webhook

import (
	"context"
	"errors"
)

// Subscriber is a subscriber that listen events from a webhook set
// at the end of the pipeline.
type Subscriber struct {
	RouteLogger *RouteLogger
	keys        map[string]struct{}
}

// Subscribe subscribes to the given keys.
// It satisfies the e2e.Subscriber interface.
// Keys should be an event messageId.
func (s *Subscriber) Subscribe(keys ...string) error {
	var err error
	defer func() {
		if err != nil {
			s.Unsubscribe(keys...)
		}
	}()

	if s.keys == nil {
		s.keys = make(map[string]struct{})
	}

	for _, key := range keys {
		if err = s.RouteLogger.Sent(key); err != nil {
			return err
		}
		s.keys[key] = struct{}{}
	}
	return nil
}

// Unsubscribe satisfies the e2e.Subscriber interface.
func (s *Subscriber) Unsubscribe(keys ...string) {
	for _, key := range keys {
		delete(s.keys, key)
		s.RouteLogger.Delete(key)
	}
}

// WaitN satisfies the e2e.Subscriber interface.
func (s *Subscriber) WaitN(ctx context.Context, n int) ([]string, error) {
	if len(s.keys) == 0 {
		return nil, errors.New("no subscribed keys")
	}

	keys := make([]string, 0, len(s.keys))
	for k := range s.keys {
		keys = append(keys, k)
	}

	requests, err := s.RouteLogger.WaitN(ctx, n, keys...)
	if err != nil {
		return nil, err
	}

	events := make([]string, len(requests))
	for i, req := range requests {
		events[i] = req.Body
	}
	return events, nil
}
