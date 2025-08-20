package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"
)

// TODO: add timeout on http request
// TODO: add logger to application
// TODO: add metrics function to the pipeline
// TODO: add configs
// TODO: add recover
// TODO: add tests

type SecurityReport struct {
	HeadersSecure bool
	TLSCertValid  bool
}

type Result struct {
	Response           *http.Response
	SecurityReport     SecurityReport
	PerformanceMetrics PerformanceMetrics
	Err                error
}

type traceTimings struct {
	dnsStart     time.Time
	dnsDone      time.Time
	connectStart time.Time
	connectDone  time.Time
	tlsStart     time.Time
	tlsDone      time.Time
	gotConn      time.Time
}

type PerformanceMetrics struct {
	URL           string
	DNSLookup     time.Duration
	TCPConnection time.Duration
	TLSHandshake  time.Duration
	Total         time.Duration
	StatusCode    int
	ContentLength int
}

func fetch(ctx context.Context, url string) Result {
	var timings traceTimings
	var metrics PerformanceMetrics

	trace := &httptrace.ClientTrace{
		DNSStart: func(di httptrace.DNSStartInfo) {
			timings.dnsStart = time.Now()
		},
		DNSDone: func(di httptrace.DNSDoneInfo) {
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
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Result{Err: err}
	}

	req = req.WithContext(httptrace.WithClientTrace(ctx, trace))

	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}

	start := time.Now()
	resp, err := client.Do(req)
	metrics.Total = time.Since(start)
	if err != nil {
		return Result{Err: err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{Err: err}
	}
	metrics.ContentLength = len(body)

	metrics.StatusCode = resp.StatusCode
	metrics.URL = url

	fmt.Println(url)
	return Result{Response: resp, Err: nil, PerformanceMetrics: metrics}
}

func generator(ctx context.Context, urls ...string) chan Result {
	result := make(chan Result)
	wg := &sync.WaitGroup{}
	sem := make(chan struct{}, 5)

	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() {
				<-sem
			}()

			select {
			case <-ctx.Done():
				return
			case result <- fetch(ctx, url):

			}
		}(url)
	}

	go func() {
		wg.Wait()
		close(result)
		close(sem)
	}()

	return result
}

func checkSecureHeader(headers http.Header) bool {
	return headers.Get("Content-Security-Policy") != ""
}

func checkCertValid(tls *tls.ConnectionState) bool {
	if tls == nil {
		return false
	}
	return len(tls.PeerCertificates) > 0 &&
		time.Now().Before(tls.PeerCertificates[0].NotAfter)
}

func checkSecurity(ctx context.Context, ch chan Result) chan Result {
	result := make(chan Result)

	go func() {
		defer close(result)

		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-ch:
				if !ok {
					return
				}
				if v.Response == nil { // I dont know if I can do that
					v.SecurityReport = SecurityReport{false, false}
					result <- v
					continue
				}
				h := checkSecureHeader(v.Response.Header)
				t := checkCertValid(v.Response.TLS)
				v.SecurityReport = SecurityReport{HeadersSecure: h, TLSCertValid: t}

				result <- v
			}
		}
	}()

	return result
}

func main() {
	urls := []string{"https://google.com", "https://httpstat.us/400", "https://youtube.com", "https://twitch.tv"}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for v := range checkSecurity(ctx, generator(ctx, urls...)) {
		if v.Err != nil {
			fmt.Println(v.Err.Error())
			continue
		}
		fmt.Println(v)
	}
}
