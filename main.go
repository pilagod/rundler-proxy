package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"reflect"
	"slices"
)

var (
	rundlerV06Proxy *httputil.ReverseProxy
	entryPointsV06  []string

	rundlerV07Proxy *httputil.ReverseProxy
	entryPointsV07  []string

	entryPoints []string
)

func main() {
	rundlerV06Url, err := url.Parse(os.Getenv("RUNDLER_V0_6"))
	if err != nil {
		panic("Invalid url: RUNDLER_V0_6")
	}
	rundlerV06Proxy = httputil.NewSingleHostReverseProxy(rundlerV06Url)

	entryPointsV06, err = getSupportedEntryPoints(rundlerV06Url)
	if err != nil {
		panic(err)
	}
	log.Println("RundlerV06:", rundlerV06Url.String())
	log.Println("EntryPointsV06:", entryPointsV06)

	rundlerV07Url, err := url.Parse(os.Getenv("RUNDLER_V0_7"))
	if err != nil {
		panic("Invalid url: RUNDLER_V0_7")
	}
	rundlerV07Proxy = httputil.NewSingleHostReverseProxy(rundlerV07Url)

	entryPointsV07, err = getSupportedEntryPoints(rundlerV07Url)
	if err != nil {
		panic(err)
	}
	log.Println("RunderV07:", rundlerV07Url.String())
	log.Println("EntryPointsV07:", entryPointsV07)

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
		body := bytes.NewBuffer(nil)

		var req JSONRPCRequest
		if err := json.NewDecoder(io.TeeReader(r.Body, body)).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// eth_supportedEntryPoints
		if req.Method == "eth_supportedEntryPoints" {
			res := JSONRPCResponse{
				ID:      req.ID,
				JSONRPC: req.JSONRPC,
				Result:  append(entryPointsV06, entryPointsV07...),
			}
			var body bytes.Buffer
			if err := json.NewEncoder(&body).Encode(res); err != nil {
				log.Fatal(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			data := body.Bytes()
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
			w.WriteHeader(http.StatusOK)
			w.Write(body.Bytes())
			return
		}

		// Restore request body
		r.Body = io.NopCloser(body)

		// eth_chainId
		if req.Method == "eth_chainId" {
			rundlerV07Proxy.ServeHTTP(w, r)
			return
		}

		// EntryPoint locates at the first param
		// - debug_bundler_dumpMempool
		if req.Method == "debug_bundler_dumpMempool" ||
			req.Method == "debug_bundler_dumpReputation" {
			entryPoint, ok := req.Params[0].(string)
			if !ok || !isEntryPointSupported(entryPoint) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if isEntryPointV06(entryPoint) {
				rundlerV06Proxy.ServeHTTP(w, r)
				return
			}
			rundlerV07Proxy.ServeHTTP(w, r)
			return
		}

		// EntryPoint locates at the second param
		// - eth_sendUserOperation
		// - eth_estimateUserOperationGas
		// - debug_bundler_setReputation
		if req.Method == "eth_sendUserOperation" ||
			req.Method == "eth_estimateUserOperationGas" ||
			req.Method == "debug_bundler_setReputation" {
			entryPoint, ok := req.Params[1].(string)
			if !ok || !isEntryPointSupported(entryPoint) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if isEntryPointV06(entryPoint) {
				rundlerV06Proxy.ServeHTTP(w, r)
				return
			}
			rundlerV07Proxy.ServeHTTP(w, r)
			return
		}

		// Data locates at specific bundler
		if req.Method == "eth_getUserOperationByHash" ||
			req.Method == "eth_getUserOperationReceipt" {
			// Try v06 first
			rV06 := r.Clone(r.Context())
			rV06.Body = io.NopCloser(body)
			wV06 := NewProxyResponseWriter()
			rundlerV06Proxy.ServeHTTP(wV06, rV06)
			result, err := wV06.ReadJSONRPCResponse()
			if err != nil || result.Result != nil {
				wV06.Dump(w)
				return
			}
			// Fallback to v07
			rundlerV07Proxy.ServeHTTP(w, r)
			return
		}

		// Fanout to all bundlers
		// - debug_bundler_clearState
		// - debug_bundler_sendBundleNow
		// - debug_bundler_setBundlingMode
		if req.Method == "debug_bundler_clearState" ||
			req.Method == "debug_bundler_sendBundleNow" ||
			req.Method == "debug_bundler_setBundlingMode" {
			rV06 := r.Clone(r.Context())
			rV06.Body = io.NopCloser(body)
			wV06 := NewProxyResponseWriter()
			rundlerV06Proxy.ServeHTTP(wV06, rV06)
			// Only use v07 response
			rundlerV07Proxy.ServeHTTP(w, r)
		}
	}
}

func isEntryPointSupported(entryPoint string) bool {
	return slices.Contains(append(entryPointsV06, entryPointsV07...), entryPoint)
}

func isEntryPointV06(entryPoint string) bool {
	return slices.Contains(entryPointsV06, entryPoint)
}

func isEntryPointV07(entryPoint string) bool {
	return slices.Contains(entryPointsV07, entryPoint)
}
