package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

func main() {
	rundlerV06Url, err := url.Parse(os.Getenv("RUNDLER_V0_6"))
	if err != nil {
		panic("Invalid url: RUNDLER_V0_6")
	}
	rundlerV06Proxy := httputil.NewSingleHostReverseProxy(rundlerV06Url)

	mux := http.NewServeMux()
	mux.HandleFunc("/", createProxyHandler(rundlerV06Proxy))

	log.Println("Listening on :3000")
	if err = http.ListenAndServe(":3000", mux); err != nil {
		panic(err)
	}
}

type JSONRPCRequest struct {
	ID      int           `json:"id"`
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

func createProxyHandler(
	rundlerV06Proxy *httputil.ReverseProxy,
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body := new(bytes.Buffer)
		bodyReader := io.TeeReader(r.Body, body)

		var jsonRPCRequest JSONRPCRequest
		if err := json.NewDecoder(bodyReader).Decode(&jsonRPCRequest); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// Restore request body
		r.Body = io.NopCloser(body)

		wV06 := NewProxyResponseWriter()
		rundlerV06Proxy.ServeHTTP(wV06, r)

		for key, values := range wV06.Header() {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(wV06.StatusCode)
		if _, err := w.Write(wV06.Body.Bytes()); err != nil {
			log.Fatal(err)
		}
	}
}
