package webhook

import (
	"time"

	"github.com/segmentio/events"
)

type Request struct {
	// The remote IP address (with port)
	RemoteAddr string

	Method string
	Proto  string
	RawURL string

	Header map[string][]string
	Body   string

	ReceivedAt time.Time
}

type Logger interface {
	// TODO consider adding error
	Record(r Request)
}

type StdLogger struct {
}

func (s StdLogger) Record(r Request) {
	events.Log("%{request}+v", r)
}

func NewChanLogger(rc chan<- Request) ChanLogger {
	return ChanLogger{
		rc: rc,
	}
}

type ChanLogger struct {
	rc chan<- Request
}

func (s ChanLogger) Record(r Request) {
	// Requests are dropped if writing chan would block
	select {
	case s.rc <- r:
	default:
		events.Debug("chan full, dropping request")
	}
}
