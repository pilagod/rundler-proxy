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
	"reflect"
)

var (
	rundlerV06Proxy *httputil.ReverseProxy
	entryPointsV06  []string
)

func main() {
	rundlerV06Url, err := url.Parse(os.Getenv("RUNDLER_V0_6"))
	if err != nil {
		panic("Invalid url: RUNDLER_V0_6")
	}
	log.Println("RunderV06:", rundlerV06Url.String())
	rundlerV06Proxy = httputil.NewSingleHostReverseProxy(rundlerV06Url)

	entryPointsV06, err := getSupportedEntryPoints(rundlerV06Url)
	if err != nil {
		panic(err)
	}
	log.Println("EntryPointsV06:", entryPointsV06)

	mux := http.NewServeMux()
	mux.HandleFunc("/", createProxyHandler())

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

type JSONRPCResponse struct {
	ID      int         `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
}

func getSupportedEntryPoints(url *url.URL) (entryPoints []string, err error) {
	r := JSONRPCRequest{
		ID:      1,
		JSONRPC: "2.0",
		Method:  "eth_supportedEntryPoints",
		Params:  make([]interface{}, 0),
	}
	var data bytes.Buffer
	if err = json.NewEncoder(&data).Encode(r); err != nil {
		return
	}
	res, err := http.Post(url.String(), "application/json", &data)
	if err != nil {
		return
	}
	var result JSONRPCResponse
	if err = json.NewDecoder(res.Body).Decode(&result); err != nil {
		return
	}
	v := reflect.ValueOf(result.Result)
	if v.IsZero() {
		return
	}
	entryPoints = make([]string, v.Len())
	for i := 0; i < v.Len(); i++ {
		entryPoints[i] = v.Index(i).Interface().(string)
	}
	return
}

func createProxyHandler() func(http.ResponseWriter, *http.Request) {
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
