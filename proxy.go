package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
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

func (w *ProxyResponseWriter) Dump(dest http.ResponseWriter) error {
	for key, values := range w.Header() {
		for _, value := range values {
			dest.Header().Add(key, value)
		}
	}
	dest.WriteHeader(w.StatusCode)
	data := w.Body.Bytes()
	dest.Header().Set("Content-Length", fmt.Sprint(len(data)))
	_, err := dest.Write(data)
	if err != nil {
		log.Fatal(err)
	}
	return err
}

func (w *ProxyResponseWriter) ReadJSONRPCResponse() (result JSONRPCResponse, err error) {
	if err = json.NewDecoder(bytes.NewReader(w.Body.Bytes())).Decode(&result); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	return
}
