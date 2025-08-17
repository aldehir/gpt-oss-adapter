package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Cache interface {
	Put(key string, item ReasoningItem)
	Get(key string) (ReasoningItem, bool)
}

type Adapter struct {
	Target string
	mux    *http.ServeMux
	client *http.Client
	cache  Cache
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

func (a *Adapter) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	var requestData map[string]any
	if err := json.Unmarshal(requestBody, &requestData); err != nil {
		http.Error(w, "Failed to unmarshal request", http.StatusInternalServerError)
		return
	}

	a.injectReasoningFromCache(requestData)

	modifiedRequestBody, err := json.Marshal(requestData)
	if err != nil {
		http.Error(w, "Failed to marshal modified request", http.StatusInternalServerError)
		return
	}

	targetURL, err := url.Parse(a.Target)
	if err != nil {
		http.Error(w, "Invalid target URL", http.StatusInternalServerError)
		return
	}

	targetURL.Path = r.URL.Path
	targetURL.RawQuery = r.URL.RawQuery

	req, err := http.NewRequest(r.Method, targetURL.String(), bytes.NewReader(modifiedRequestBody))
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

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		a.handleChatCompletionsBlocking(w, resp)
	} else if strings.Contains(contentType, "text/event-stream") {
		a.handleChatCompletionsStreaming(w, resp)
	} else {
		for name, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

func (a *Adapter) handleChatCompletionsBlocking(w http.ResponseWriter, resp *http.Response) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response body", http.StatusInternalServerError)
		return
	}

	var responseData map[string]any
	if err := json.Unmarshal(body, &responseData); err != nil {
		http.Error(w, "Failed to unmarshal response", http.StatusInternalServerError)
		return
	}

	a.extractAndCacheReasoning(responseData)

	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func (a *Adapter) injectReasoningFromCache(requestData map[string]any) {
	messages, ok := requestData["messages"].([]any)
	if !ok {
		return
	}

	for _, msg := range messages {
		message, ok := msg.(map[string]any)
		if !ok {
			continue
		}

		role, ok := message["role"].(string)
		if !ok || role != "assistant" {
			continue
		}

		toolCalls, ok := message["tool_calls"].([]any)
		if !ok || len(toolCalls) == 0 {
			continue
		}

		for _, tc := range toolCalls {
			toolCall, ok := tc.(map[string]any)
			if !ok {
				continue
			}

			id, ok := toolCall["id"].(string)
			if !ok {
				continue
			}

			if item, found := a.cache.Get(id); found {
				message["reasoning_content"] = item.Content
				break
			}
		}
	}
}

func (a *Adapter) extractAndCacheReasoning(responseData map[string]any) {
	choices, ok := responseData["choices"].([]any)
	if !ok || len(choices) == 0 {
		return
	}

	choice, ok := choices[0].(map[string]any)
	if !ok {
		return
	}

	message, ok := choice["message"].(map[string]any)
	if !ok {
		return
	}

	toolCalls, ok := message["tool_calls"].([]any)
	if !ok || len(toolCalls) == 0 {
		return
	}

	reasoningContent, ok := message["reasoning_content"].(string)
	if !ok {
		return
	}

	toolCall, ok := toolCalls[0].(map[string]any)
	if !ok {
		return
	}

	id, ok := toolCall["id"].(string)
	if !ok {
		return
	}

	item := ReasoningItem{
		ID:      id,
		Content: reasoningContent,
	}
	a.cache.Put(id, item)
}

func (a *Adapter) handleChatCompletionsStreaming(w http.ResponseWriter, resp *http.Response) {
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func NewAdapter(target string, cache Cache) *Adapter {
	mux := http.NewServeMux()
	adapter := &Adapter{
		Target: target,
		mux:    mux,
		client: &http.Client{},
		cache:  cache,
	}

	mux.HandleFunc("/", adapter.handleDefault)

	return adapter
}
