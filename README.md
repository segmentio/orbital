orbital
-------

[![Build](https://circleci.com/gh/segmentio/orbital.svg?style=shield&circle-token=d06625b898a4090cd613386530e9296a286b6c2b)](https://circleci.com/gh/segmentio/orbital)
[![GoDoc](https://godoc.org/github.com/segmentio/orbital?status.svg)](https://godoc.org/github.com/segmentio/orbital)
[![Go Report Card](https://goreportcard.com/badge/github.com/segmentio/orbital#)](https://goreportcard.com/report/github.com/segmentio/orbital)
[![License](https://img.shields.io/badge/license-MIT-5B74AD.svg)](https://github.com/segmentio/orbital/blob/master/LICENSE)

Orbital is a test framework which enables a developer to write end to end tests
just like one would writing unit tests.  We do this by effectively copying the
`testing.T` API and registering tests to be run periodically on a configured
schedule.

This package is not yet API stable.  Use with the understanding that it might
change as time goes on.

### motivation

Writing tests should be easy.  This includes oft-neglected end-to-end tests
which provide arguably the most value.  End to end tests can be used for
functional verification before a release, alerts when your site isn't behaving
correctly, or simply just providing metrics about your site or service.

### usage

The goal is to make writing end-to-end tests simple and to take the effort out
of building these systems.  To enable that, a number of packages are provided
to aid in this effort.  The webhook package provides a simple way to receieve
notifications of received events.  With those packages together, we can write
elegant tests like the following.


```
type Harness struct {
	RouteLogger *webhook.RouteLogger
}

func (h *Harness) OrbitalSmoke(ctx context.Context, o *orbital.O) {
	s := sender{APIKey: "super-private-api-key"}
	// Send request to API for handling
	id := s.send([]byte(tmpl))

	// tell the webhook we're waiting to recieve this message
	err := h.RouteLogger.Sent(id)
	if err != nil {
		o.Errorf("%{error}v", err)
		return
	}
	// Cleanup
	defer h.RouteLogger.Delete(id)

	// wait for this message to be received by the webhook
	_, err = h.RouteLogger.Wait(ctx, id)
	if err != nil {
		o.Errorf("%{error}v", err)
		return
	}
}
```
