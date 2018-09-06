package webhook

import (
	"io/ioutil"
	"net/http"
	"time"
)

type Webhook struct {
	log Logger
}

type Config struct {
	Logger Logger
}

func New(c Config, opts ...func(*Webhook)) *Webhook {
	ret := &Webhook{
		log: c.Logger,
	}
	for _, o := range opts {
		o(ret)
	}
	return ret
}

func (h *Webhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rdr := http.MaxBytesReader(w, r.Body, 30000)
	defer rdr.Close()

	bs, err := ioutil.ReadAll(rdr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	h.log.Record(Request{
		RemoteAddr: r.RemoteAddr,
		Method:     r.Method,
		Proto:      r.Proto,
		RawURL:     r.URL.String(),
		Header:     cloneHeader(r.Header),
		Body:       string(bs),
		ReceivedAt: time.Now().UTC(),
	})
}

func (h *Webhook) Close() error {
	return nil
}

func cloneHeader(h http.Header) http.Header {
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}
