package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"os"
	"sync"
	"time"
)

// TODO: add logger to application
// TODO: add configs
// TODO: add tests

type SecurityReport struct {
	HeadersSecure bool
	TLSCertValid  bool
}

type Result struct {
	Response           *http.Response     `json:"-"`
	SecurityReport     SecurityReport     `json:"security"`
	PerformanceMetrics PerformanceMetrics `json:"metrics"`
	Err                error              `json:"error"`
}

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

type PerformanceMetrics struct {
	URL           string        `json:"url"`
	DNSLookup     time.Duration `json:"dns_lookup"`
	TCPConnection time.Duration `json:"tcp_connection"`
	TLSHandshake  time.Duration `json:"tls_handshake"`
	TTFB          time.Duration `json:"ttfb"`
	Total         time.Duration `json:"total"`
	StatusCode    int           `json:"status_code"`
	ContentLength int           `json:"content_length"`
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

func fetch(ctx context.Context, url string) Result {
	var timings traceTimings
	var metrics PerformanceMetrics

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
		return Result{Err: err}
	}

	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return Result{Err: err}
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

	return Result{Response: resp, PerformanceMetrics: metrics}
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results := checkSecurity(ctx, generator(ctx, urls...))

	var allResults []Result

	for v := range results {
		if v.Err != nil {
			fmt.Println(v.Err.Error())
			continue
		}
		allResults = append(allResults, v)
		fmt.Println(v)
	}

	if err := exportJSON(allResults); err != nil {
		fmt.Printf("JSON export error: %v\n", err)
	}
}

func exportJSON(data []Result) error {
	content, err := json.Marshal(data)
	if err != nil {
		return err
	}

	err = os.WriteFile("results.json", content, 0644)
	if err != nil {
		return err
	}
	fmt.Println(data)
	return nil
}
