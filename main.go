package main

import (
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
	mux.HandleFunc("/", createProxyHandler(*rundlerV06Proxy))

	err = http.ListenAndServe(":3000", mux)
	log.Fatal(err)
}

func createProxyHandler(
	rundlerV06Proxy httputil.ReverseProxy,
) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		rundlerV06Proxy.ServeHTTP(w, r)
	}
}
