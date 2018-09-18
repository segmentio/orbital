package example

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/segmentio/orbital/orbital"
	"github.com/segmentio/orbital/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/yields/phony/pkg/phony"
)

type Harness struct {
	API    string
	Waiter *webhook.RouteLogger
}

type event struct {
	Email     string    `json:"email"`
	Timestamp time.Time `json:"timestamp"`
	ID        string    `json:"id"`
	Processed bool      `json:"processed"`
}

func (h Harness) OrbitalSmoke(ctx context.Context, o *orbital.O) {
	evt := event{
		Email:     phony.Get("email"),
		ID:        phony.Get("ksuid"),
		Timestamp: time.Now().UTC(),
	}
	// Mark the event as Sent.
	err := h.Waiter.Sent(evt.ID)
	assert.NoError(o, err, "error marking sent")
	// Cleanup after we're done.
	defer h.Waiter.Delete(evt.ID)
	assert.NoError(o, send(h.API, evt), "sending event shouldn't fail")
	// Block until the event has been received
	r, err := h.Waiter.Wait(ctx, evt.ID)
	assert.NoError(o, err, "error waiting")
	var recv event
	err = json.Unmarshal([]byte(r.Body), &recv)
	assert.NoError(o, err, "error unmarshaling")
	assert.True(o, recv.Processed, "processed should be set to true")
}

func send(api string, e event) error {
	bs := bytes.NewBuffer(nil)
	enc := json.NewEncoder(bs)
	err := enc.Encode(e)
	if err != nil {
		return err
	}
	resp, err := http.Post(api, "application/json", bs)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("non-200 status")
	}
	return nil
}
