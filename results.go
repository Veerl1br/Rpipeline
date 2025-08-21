package rpipeline

import "net/http"

type Result struct {
	Response           *http.Response     `json:"-"`
	SecurityReport     SecurityReport     `json:"security"`
	PerformanceMetrics PerformanceMetrics `json:"metrics"`
	Err                error              `json:"error"`
}
