package main

import (
	"net/http"
)

type Adapter struct{}

func (a *Adapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Hello from adapter"}`))
}

func NewAdapter() *Adapter {
	return &Adapter{}
}
