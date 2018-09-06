package webhook

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRouteLogger(t *testing.T) {
	rl := NewRouteLogger(bodyMatch)

	rl.Sent("blah")

	wr := Request{
		Method: "FOO",
		Body:   "blah",
	}
	rl.Record(wr)

	x, err := rl.Wait(context.Background(), "blah")
	assert.Equal(t, wr, x, "received request should equal sent")
	assert.NoError(t, err)
}

func TestRouteLoggerWaitFor(t *testing.T) {
	tests := []struct {
		scenario   string
		in         []string
		wait       []string
		err        bool
		timeoutErr bool
	}{
		{
			scenario: "wait one",
			in:       []string{"hello"},
			wait:     []string{"hello"},
		},
		{
			scenario: "wait Many",
			in: []string{
				"hello",
				"world",
				"orbital",
				"roxx",
			},
			wait: []string{
				"hello",
				"world",
				"orbital",
				"roxx",
			},
		},
		{
			scenario: "wait not expected",
			in: []string{
				"hello",
				"world",
				"orbital",
				"roxx",
			},
			wait: []string{
				"orbital",
				"roxx",
				"boo",
			},
			err: true,
		},
		{
			scenario: "wait timeout",
			in: []string{
				"hello",
			},
			wait: []string{
				"hello",
				"orbital",
			},
			timeoutErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			rl := NewRouteLogger(bodyMatch)

			for _, in := range test.in {
				err := rl.Sent(in)
				require.NoError(t, err)

				rl.Record(Request{
					Method: "Test",
					Body:   in,
				})
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			requests, err := rl.WaitN(ctx, len(test.wait), test.in...)
			if test.timeoutErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, len(test.wait), len(requests))

			out := make([]string, len(test.wait))
			for i, r := range requests {
				out[i] = r.Body
			}

			if test.err {
				require.NotSubset(t, out, test.wait)
				return
			}
			require.Subset(t, out, test.wait)

		})
	}
}

func bodyMatch(wr Request) string {
	return wr.Body
}
