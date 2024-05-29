package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

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

func (r *JSONRPCResponse) Write(w http.ResponseWriter) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(r); err != nil {
		log.Fatal(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	data := buf.Bytes()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
