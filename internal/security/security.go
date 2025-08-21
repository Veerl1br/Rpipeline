package security

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	rpipeline "github.com/Veerl1br/Rpipeline"
)

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

func CheckSecurity(ctx context.Context, ch chan rpipeline.Result) chan rpipeline.Result {
	result := make(chan rpipeline.Result)

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
					v.SecurityReport = rpipeline.SecurityReport{HeadersSecure: false, TLSCertValid: false}
					result <- v
					continue
				}
				h := checkSecureHeader(v.Response.Header)
				t := checkCertValid(v.Response.TLS)
				v.SecurityReport = rpipeline.SecurityReport{HeadersSecure: h, TLSCertValid: t}

				result <- v
			}
		}
	}()

	return result
}
