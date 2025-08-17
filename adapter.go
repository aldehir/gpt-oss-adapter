package main

import (
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Adapter struct {
	Target string
	mux    *http.ServeMux
	client *http.Client
}

func (a *Adapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

func (a *Adapter) handleDefault(w http.ResponseWriter, r *http.Request) {
	targetURL, err := url.Parse(a.Target)
	if err != nil {
		http.Error(w, "Invalid target URL", http.StatusInternalServerError)
		return
	}

	targetURL.Path = r.URL.Path
	targetURL.RawQuery = r.URL.RawQuery

	req, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	for name, values := range r.Header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	resp, err := a.client.Do(req)
	if err != nil {
		http.Error(w, "Failed to proxy request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func NewAdapter(target string) *Adapter {
	mux := http.NewServeMux()
	adapter := &Adapter{
		Target: target,
		mux:    mux,
		client: &http.Client{},
	}

	mux.HandleFunc("/", adapter.handleDefault)

	return adapter
}
