package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// TODO: add timeout on http request
// TODO: add logger to application
// TODO: add metrics function to the pipeline

type SecurityReport struct {
	HeadersSecure bool
	TLSCertValid  bool
}

type Result struct {
	res       *http.Response
	secreport SecurityReport
	err       error
}

func fetch(url string) Result {
	//time.Sleep(2 * time.Second) // long work emission

	resp, err := http.Get(url)
	if err != nil {
		return Result{res: resp, err: err}
	}
	fmt.Println(url)
	return Result{res: resp, err: err}
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
			case result <- fetch(url):

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

// Вспомогательные функции проверки безопасности
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
				if v.res == nil { // I dont know if I can do that
					v.secreport = SecurityReport{false, false}
					result <- v
					continue
				}
				h := checkSecureHeader(v.res.Header)
				t := checkCertValid(v.res.TLS)
				v.secreport = SecurityReport{HeadersSecure: h, TLSCertValid: t}

				result <- v
			}
		}
	}()

	return result
}

func main() {
	urls := []string{"https://google.com", "https://httpstat.us/400", "https://youtube.com", "https://twitch.tv"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for v := range checkSecurity(ctx, generator(ctx, urls...)) {
		if v.err != nil {
			fmt.Println(v.err.Error())
			continue
		}
		fmt.Println(v)
	}
}
