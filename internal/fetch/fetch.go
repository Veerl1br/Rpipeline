package fetch

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptrace"
	"time"

	rpipeline "github.com/Veerl1br/Rpipeline"
)

type traceTimings struct {
	dnsStart     time.Time
	dnsDone      time.Time
	connectStart time.Time
	connectDone  time.Time
	tlsStart     time.Time
	tlsDone      time.Time
	gotConn      time.Time
	firstByte    time.Time
}

var defaultTransport = &http.Transport{
	Proxy:                 http.ProxyFromEnvironment,
	MaxIdleConns:          100,
	MaxConnsPerHost:       32,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

var httpClient = &http.Client{
	Transport: defaultTransport,
	Timeout:   15 * time.Second,
}

func Fetch(ctx context.Context, url string) rpipeline.Result {
	var timings traceTimings
	var metrics rpipeline.PerformanceMetrics

	metrics.URL = url

	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			timings.dnsStart = time.Now()
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			timings.dnsDone = time.Now()
			metrics.DNSLookup = timings.dnsDone.Sub(timings.dnsStart)
		},
		ConnectStart: func(network, addr string) {
			timings.connectStart = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			timings.connectDone = time.Now()
			metrics.TCPConnection = timings.connectDone.Sub(timings.connectStart)
		},
		TLSHandshakeStart: func() {
			timings.tlsStart = time.Now()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			timings.tlsDone = time.Now()
			metrics.TLSHandshake = timings.tlsDone.Sub(timings.tlsStart)
		},
		GotConn: func(info httptrace.GotConnInfo) {
			timings.gotConn = time.Now()
		},
		GotFirstResponseByte: func() {
			timings.firstByte = time.Now()
		},
	}

	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), http.MethodGet, url, nil)
	if err != nil {
		return rpipeline.Result{Err: err}
	}

	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return rpipeline.Result{Err: err}
	}
	defer resp.Body.Close()

	if !timings.firstByte.IsZero() {
		metrics.TTFB = timings.firstByte.Sub(start)
	}

	n, _ := io.Copy(io.Discard, resp.Body)
	if resp.ContentLength >= 0 {
		metrics.ContentLength = int(resp.ContentLength)
	} else {
		metrics.ContentLength = int(n)
	}
	metrics.StatusCode = resp.StatusCode
	metrics.Total = time.Since(start)

	return rpipeline.Result{Response: resp, PerformanceMetrics: metrics}
}
