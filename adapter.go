package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/aldehir/gpt-oss-adapter/providers/types"
)

type Cache interface {
	Put(key string, item ReasoningItem)
	Get(key string) (ReasoningItem, bool)
}

type Adapter struct {
	Target   string
	Provider types.Provider
	mux      *http.ServeMux
	client   *http.Client
	cache    Cache
	logger   *slog.Logger
}

func NewAdapter(target string, cache Cache, logger *slog.Logger, provider types.Provider) *Adapter {
	mux := http.NewServeMux()
	adapter := &Adapter{
		Target:   target,
		Provider: provider,
		mux:      mux,
		client:   &http.Client{},
		cache:    cache,
		logger:   logger,
	}

	mux.HandleFunc("/v1/chat/completions", adapter.handleChatCompletions)
	mux.HandleFunc("/chat/completions", adapter.handleChatCompletions)
	mux.HandleFunc("/", adapter.handleDefault)

	return adapter
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

	targetURL.Path = strings.TrimSuffix(targetURL.Path, "/") + r.URL.Path
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

	req.Header.Del("Accept-Encoding")

	if req.Header.Get("X-Forwarded-For") == "" {
		if clientIP := getClientIP(r); clientIP != "" {
			req.Header.Set("X-Forwarded-For", clientIP)
		}
	}

	resp, err := a.client.Do(req)
	if err != nil {
		http.Error(w, "Failed to proxy request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for name, values := range resp.Header {
		if name == "Content-Length" {
			continue
		}
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (a *Adapter) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	a.logger.Info("handling chat completions request", "method", r.Method, "path", r.URL.Path)

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		a.logger.Error("failed to read request body", "error", err)
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	var requestData map[string]any
	if err := json.Unmarshal(requestBody, &requestData); err != nil {
		a.logger.Error("failed to unmarshal request", "error", err)
		http.Error(w, "Failed to unmarshal request", http.StatusInternalServerError)
		return
	}

	a.injectReasoningFromCache(requestData)
	a.injectReasoningEffort(requestData)

	modifiedRequestBody, err := json.Marshal(requestData)
	if err != nil {
		a.logger.Error("failed to marshal modified request", "error", err)
		http.Error(w, "Failed to marshal modified request", http.StatusInternalServerError)
		return
	}

	targetURL, err := url.Parse(a.Target)
	if err != nil {
		a.logger.Error("invalid target URL", "target", a.Target, "error", err)
		http.Error(w, "Invalid target URL", http.StatusInternalServerError)
		return
	}

	targetURL.Path = strings.TrimSuffix(targetURL.Path, "/") + r.URL.Path
	targetURL.RawQuery = r.URL.RawQuery

	a.logger.Debug("proxying request to target", "target", targetURL.String())

	req, err := http.NewRequest(r.Method, targetURL.String(), bytes.NewReader(modifiedRequestBody))
	if err != nil {
		a.logger.Error("failed to create request", "error", err)
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	for name, values := range r.Header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	req.Header.Del("Accept-Encoding")

	if req.Header.Get("X-Forwarded-For") == "" {
		if clientIP := getClientIP(r); clientIP != "" {
			req.Header.Set("X-Forwarded-For", clientIP)
		}
	}

	resp, err := a.client.Do(req)
	if err != nil {
		a.logger.Error("failed to proxy request", "error", err)
		http.Error(w, "Failed to proxy request", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	a.logger.Debug("received response", "status", resp.StatusCode, "content-type", contentType)

	if strings.Contains(contentType, "text/event-stream") {
		a.logger.Debug("handling streaming response")
		a.handleChatCompletionsStreaming(w, resp)
	} else {
		a.logger.Debug("handling blocking response")
		a.handleChatCompletionsBlocking(w, resp)
	}
}

func (a *Adapter) handleChatCompletionsBlocking(w http.ResponseWriter, resp *http.Response) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		a.logger.Error("failed to read response body", "error", err)
		http.Error(w, "Failed to read response body", http.StatusInternalServerError)
		return
	}

	var responseData map[string]any
	if err := json.Unmarshal(body, &responseData); err != nil {
		a.logger.Error("failed to unmarshal response", "error", err)
		http.Error(w, "Failed to unmarshal response", http.StatusInternalServerError)
		return
	}

	a.extractAndCacheReasoning(responseData)
	a.transformReasoningContentToReasoning(responseData)

	modifiedBody, err := json.Marshal(responseData)
	if err != nil {
		a.logger.Error("failed to marshal modified response", "error", err)
		http.Error(w, "Failed to marshal modified response", http.StatusInternalServerError)
		return
	}

	for name, values := range resp.Header {
		if name == "Content-Length" {
			continue
		}
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(modifiedBody)
}

func (a *Adapter) transformReasoningContentToReasoning(responseData map[string]any) {
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

	if reasoningContent, ok := message[a.Provider.Reasoning].(string); ok {
		message["reasoning"] = reasoningContent
		delete(message, a.Provider.Reasoning)
		a.logger.Debug("transformed reasoning field", "from", a.Provider.Reasoning, "to", "reasoning")
	}
}

func (a *Adapter) injectReasoningFromCache(requestData map[string]any) {
	messages, ok := requestData["messages"].([]any)
	if !ok {
		return
	}

	injectedCount := 0
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
				message[a.Provider.Reasoning] = item.Content
				injectedCount++
				a.logger.Debug("injected reasoning content from cache", "tool_call_id", id, "field", a.Provider.Reasoning)
				break
			}
		}
	}

	if injectedCount > 0 {
		a.logger.Info("injected reasoning content", "count", injectedCount)
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

	reasoningContent, ok := message[a.Provider.Reasoning].(string)
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
	a.logger.Info("cached reasoning content", "tool_call_id", id, "content_length", len(reasoningContent))
}

func (a *Adapter) handleChatCompletionsStreaming(w http.ResponseWriter, resp *http.Response) {
	a.logger.Debug("starting streaming response processing")

	for name, values := range resp.Header {
		if name == "Content-Length" {
			continue
		}
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	flusher, ok := w.(http.Flusher)
	if !ok {
		a.logger.Warn("response writer does not support flushing, falling back to simple copy")
		io.Copy(w, resp.Body)
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	var reasoningContent strings.Builder
	var toolCallID string

	for scanner.Scan() {
		line := scanner.Text()
		modifiedLine := a.transformStreamingLine(line)

		w.Write([]byte(modifiedLine + "\n"))
		flusher.Flush()

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				a.logger.Debug("received [DONE] event, finalizing stream")
				if reasoningContent.Len() > 0 && toolCallID != "" {
					item := ReasoningItem{
						ID:      toolCallID,
						Content: reasoningContent.String(),
					}
					a.cache.Put(toolCallID, item)
					a.logger.Info("cached reasoning content from stream", "tool_call_id", toolCallID, "content_length", reasoningContent.Len())
				}
				continue
			}

			var eventData map[string]any
			if err := json.Unmarshal([]byte(data), &eventData); err != nil {
				continue
			}

			a.processStreamingDelta(eventData, &reasoningContent, &toolCallID)
		}
	}

	if reasoningContent.Len() > 0 && toolCallID != "" {
		item := ReasoningItem{
			ID:      toolCallID,
			Content: reasoningContent.String(),
		}
		a.cache.Put(toolCallID, item)
		a.logger.Info("cached reasoning content from stream end", "tool_call_id", toolCallID, "content_length", reasoningContent.Len())
	}

	a.logger.Debug("completed streaming response processing")
}

func (a *Adapter) transformStreamingLine(line string) string {
	if !strings.HasPrefix(line, "data: ") {
		return line
	}

	data := strings.TrimPrefix(line, "data: ")
	if data == "[DONE]" || data == "" {
		return line
	}

	var eventData map[string]any
	if err := json.Unmarshal([]byte(data), &eventData); err != nil {
		return line
	}

	choices, ok := eventData["choices"].([]any)
	if !ok || len(choices) == 0 {
		return line
	}

	choice, ok := choices[0].(map[string]any)
	if !ok {
		return line
	}

	delta, ok := choice["delta"].(map[string]any)
	if !ok {
		return line
	}

	if reasoningContent, ok := delta[a.Provider.Reasoning].(string); ok {
		delta["reasoning"] = reasoningContent
		delete(delta, a.Provider.Reasoning)

		modifiedData, err := json.Marshal(eventData)
		if err != nil {
			return line
		}
		return "data: " + string(modifiedData)
	}

	return line
}

func (a *Adapter) processStreamingDelta(eventData map[string]any, reasoningContent *strings.Builder, toolCallID *string) {
	choices, ok := eventData["choices"].([]any)
	if !ok || len(choices) == 0 {
		return
	}

	choice, ok := choices[0].(map[string]any)
	if !ok {
		return
	}

	delta, ok := choice["delta"].(map[string]any)
	if !ok {
		return
	}

	if reasoningDelta, ok := delta[a.Provider.Reasoning].(string); ok {
		reasoningContent.WriteString(reasoningDelta)
	}

	if toolCalls, ok := delta["tool_calls"].([]any); ok && len(toolCalls) > 0 {
		if toolCall, ok := toolCalls[0].(map[string]any); ok {
			if id, ok := toolCall["id"].(string); ok {
				*toolCallID = id
			}
		}
	}
}

func (a *Adapter) injectReasoningEffort(requestData map[string]any) {
	if a.Provider.ReasoningEffort == "" {
		return
	}

	if a.Provider.ReasoningEffort == "reasoning.effort" {
		return
	}

	reasoningEffort := a.getNestedField(requestData, "reasoning.effort")
	if reasoningEffort == nil {
		return
	}

	a.setNestedField(requestData, a.Provider.ReasoningEffort, reasoningEffort)
	a.deleteNestedField(requestData, "reasoning.effort")
	a.logger.Debug("injected reasoning effort", "field", a.Provider.ReasoningEffort, "value", reasoningEffort)
}

func (a *Adapter) getNestedField(data map[string]any, path string) any {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if i == len(parts)-1 {
			return current[part]
		}

		if next, ok := current[part].(map[string]any); ok {
			current = next
		} else {
			return nil
		}
	}

	return nil
}

func (a *Adapter) setNestedField(data map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}

		if _, exists := current[part]; !exists {
			current[part] = make(map[string]any)
		}

		if next, ok := current[part].(map[string]any); ok {
			current = next
		} else {
			return
		}
	}
}

func (a *Adapter) deleteNestedField(data map[string]any, path string) {
	parts := strings.Split(path, ".")
	if len(parts) == 1 {
		delete(data, parts[0])
		return
	}

	current := data
	for _, part := range parts[:len(parts)-1] {
		if next, ok := current[part].(map[string]any); ok {
			current = next
		} else {
			return
		}
	}

	delete(current, parts[len(parts)-1])
}
