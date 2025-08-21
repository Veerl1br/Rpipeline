package rpipeline

import "time"

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
