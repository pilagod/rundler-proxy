package main

import (
	"bytes"
	"net/http"
)

type ProxyResponseWriter struct {
	StatusCode int
	Body       *bytes.Buffer
	header     http.Header
}

func NewProxyResponseWriter() *ProxyResponseWriter {
	return &ProxyResponseWriter{
		Body:   bytes.NewBuffer(nil),
		header: http.Header{},
	}
}

func (w *ProxyResponseWriter) Header() http.Header {
	return w.header
}

func (w *ProxyResponseWriter) Write(b []byte) (int, error) {
	return w.Body.Write(b)
}

func (w *ProxyResponseWriter) WriteHeader(statusCode int) {
	w.StatusCode = statusCode
}
