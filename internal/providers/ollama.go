package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/deagy/lana/internal/provider"
)

// OllamaClient implements provider.Client for local Ollama instances.
type OllamaClient struct {
	endpoint   string
	model      string
	httpClient *http.Client
}

// NewOllamaClient creates a new Ollama provider client.
func NewOllamaClient(endpoint, model string) *OllamaClient {
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	// Ensure no trailing slash
	endpoint = strings.TrimSuffix(endpoint, "/")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &OllamaClient{
		endpoint:   endpoint,
		model:      model,
		httpClient: client,
	}
}

// Chat implements provider.Client.
func (c *OllamaClient) Chat(ctx context.Context, req *provider.Request) (provider.Reader, error) {
	// Build request
	reqBody := c.buildRequestBody(req)
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/api/chat", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Make request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	// Check status
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
	}

	// Return streaming reader
	return &ollamaJSONLReader{
		body:    resp.Body,
		scanner: bufio.NewScanner(resp.Body),
	}, nil
}

// Name implements provider.Client.
func (c *OllamaClient) Name() string {
	return "ollama"
}

// Model implements provider.Client.
func (c *OllamaClient) Model() string {
	return c.model
}

// SupportedModels implements provider.Client.
func (c *OllamaClient) SupportedModels(ctx context.Context) ([]provider.ModelInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.endpoint+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]provider.ModelInfo, len(result.Models))
	for i, m := range result.Models {
		models[i] = provider.ModelInfo{
			ID:   m.Name,
			Name: m.Name,
		}
	}

	return models, nil
}

type ollamaRequestBody struct {
	Model    string           `json:"model"`
	Messages []ollamaMessage  `json:"messages"`
	Stream   bool             `json:"stream"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (c *OllamaClient) buildRequestBody(req *provider.Request) interface{} {
	msgs := make([]ollamaMessage, len(req.Messages))
	for i, msg := range req.Messages {
		msgs[i] = ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	return ollamaRequestBody{
		Model:    c.model,
		Messages: msgs,
		Stream:   true,
	}
}

// ollamaJSONLReader reads JSONL responses from Ollama.
type ollamaJSONLReader struct {
	body            io.Closer
	scanner         *bufio.Scanner
	done            bool
	roleEmitted     bool
	pendingContent  string
}

// NextEvent implements provider.Reader.
func (r *ollamaJSONLReader) NextEvent(ctx context.Context) (provider.Event, error) {
	if r.done {
		return nil, io.EOF
	}

	// If we have pending content from the last emit start event, return it now
	if r.pendingContent != "" {
		content := r.pendingContent
		r.pendingContent = ""
		return &provider.MessageDeltaEvent{
			Content: content,
		}, nil
	}

	for r.scanner.Scan() {
		line := r.scanner.Text()
		if line == "" {
			continue
		}

		var msg ollamaResponseMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return nil, fmt.Errorf("parse message: %w", err)
		}

		// Emit role on first message (only once)
		if !r.roleEmitted && msg.Message.Role != "" {
			r.roleEmitted = true
			// Save any content to emit after the start event
			if msg.Message.Content != "" {
				r.pendingContent = msg.Message.Content
			}
			return &provider.MessageStartEvent{
				Role: msg.Message.Role,
			}, nil
		}

		// Emit content if present
		if msg.Message.Content != "" {
			return &provider.MessageDeltaEvent{
				Content: msg.Message.Content,
			}, nil
		}

		// Check if done
		if msg.Done {
			r.done = true
			return &provider.MessageEndEvent{
				StopReason: "stop",
			}, nil
		}
		// If nothing to emit, continue to next line
	}

	if err := r.scanner.Err(); err != nil {
		return nil, err
	}

	return nil, io.EOF
}

// Close implements provider.Reader.
func (r *ollamaJSONLReader) Close() error {
	r.done = true
	return r.body.Close()
}

type ollamaResponseMessage struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Message   struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done            bool `json:"done"`
	TotalDuration   int64 `json:"total_duration,omitempty"`
	LoadDuration    int64 `json:"load_duration,omitempty"`
	PromptEvalCount int   `json:"prompt_eval_count,omitempty"`
	EvalCount       int   `json:"eval_count,omitempty"`
}

// IsAvailable checks if Ollama is running at the endpoint.
func (c *OllamaClient) IsAvailable(ctx context.Context) bool {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.endpoint+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
