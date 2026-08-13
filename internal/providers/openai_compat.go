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

// OpenAICompatibleClient implements provider.Client for OpenAI-compatible APIs.
type OpenAICompatibleClient struct {
	endpoint   string
	apiKey     string
	model      string
	httpClient *http.Client
	headers    map[string]string
}

// NewOpenAICompatibleClient creates a new OpenAI-compatible provider.
func NewOpenAICompatibleClient(endpoint, apiKey, model string) *OpenAICompatibleClient {
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	// Ensure no trailing slash
	endpoint = strings.TrimSuffix(endpoint, "/")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &OpenAICompatibleClient{
		endpoint:   endpoint,
		apiKey:     apiKey,
		model:      model,
		httpClient: client,
		headers:    make(map[string]string),
	}
}

// SetHeader sets a custom header (e.g., for organization or project headers).
func (c *OpenAICompatibleClient) SetHeader(key, value string) {
	c.headers[key] = value
}

// Chat implements provider.Client.
func (c *OpenAICompatibleClient) Chat(ctx context.Context, req *provider.Request) (provider.Reader, error) {
	// Prepare the request
	reqBody := c.buildRequestBody(req)
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}

	// Make the request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
	}

	// Return a streaming reader
	return &openaiSSEReader{
		body:    resp.Body,
		scanner: bufio.NewScanner(resp.Body),
	}, nil
}

// Name implements provider.Client.
func (c *OpenAICompatibleClient) Name() string {
	return "openai-compat"
}

// Model implements provider.Client.
func (c *OpenAICompatibleClient) Model() string {
	return c.model
}

// SupportedModels implements provider.Client.
// Note: OpenAI-compatible endpoints don't always support model discovery.
// Placeholder for now.
func (c *OpenAICompatibleClient) SupportedModels(ctx context.Context) ([]provider.ModelInfo, error) {
	// Real implementation would call /v1/models endpoint
	return []provider.ModelInfo{
		{ID: c.model, Name: c.model},
	}, nil
}

type openaiRequestBody struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	Stream      bool            `json:"stream"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature *float32        `json:"temperature,omitempty"`
	Tools       []openaiTool    `json:"tools,omitempty"`
	ToolChoice  *string         `json:"tool_choice,omitempty"`
}

type openaiMessage struct {
	Role      string          `json:"role"`
	Content   interface{}     `json:"content"` // string or []object
	ToolID    string          `json:"tool_call_id,omitempty"`
	Name      string          `json:"name,omitempty"`
	ToolCalls []openaiToolUse `json:"tool_calls,omitempty"`
}

type openaiToolUse struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"` // "function"
	Function openaiFunction `json:"function"`
}

type openaiFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type openaiTool struct {
	Type     string               `json:"type"` // "function"
	Function openaiToolDefinition `json:"function"`
}

type openaiToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

func (c *OpenAICompatibleClient) buildRequestBody(req *provider.Request) interface{} {
	// Convert messages
	msgs := make([]openaiMessage, len(req.Messages))
	for i, msg := range req.Messages {
		msgs[i] = openaiMessage{
			Role:    msg.Role,
			Content: msg.Content,
			ToolID:  msg.ToolID,
			Name:    msg.Name,
		}

		// Convert tool uses if present
		if len(msg.ToolUse) > 0 {
			msgs[i].ToolCalls = make([]openaiToolUse, len(msg.ToolUse))
			for j, tu := range msg.ToolUse {
				msgs[i].ToolCalls[j] = openaiToolUse{
					ID:   tu.ID,
					Type: "function",
					Function: openaiFunction{
						Name:      tu.Name,
						Arguments: tu.Input,
					},
				}
			}
		}
	}

	// Convert tools
	tools := make([]openaiTool, len(req.Tools))
	for i, tool := range req.Tools {
		tools[i] = openaiTool{
			Type: "function",
			Function: openaiToolDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}
	}

	return openaiRequestBody{
		Model:       c.model,
		Messages:    msgs,
		Stream:      true,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Tools:       tools,
		ToolChoice:  req.ToolChoice,
	}
}

// openaiSSEReader reads Server-Sent Events from OpenAI-compatible endpoints.
type openaiSSEReader struct {
	body    io.Closer
	scanner *bufio.Scanner
	done    bool
}

// NextEvent implements provider.Reader.
func (r *openaiSSEReader) NextEvent(ctx context.Context) (provider.Event, error) {
	if r.done {
		return nil, io.EOF
	}

	// Read lines until we find a data: line
	for r.scanner.Scan() {
		line := r.scanner.Text()

		// Check for done marker
		if line == "data: [DONE]" {
			r.done = true
			return &provider.MessageEndEvent{StopReason: "stop"}, nil
		}

		// Look for data: prefix
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			return r.parseChunk(data)
		}

		// Empty lines are normal in SSE
		if line == "" {
			continue
		}
	}

	// Check for scan error
	if err := r.scanner.Err(); err != nil {
		return nil, err
	}

	return nil, io.EOF
}

// Close implements provider.Reader.
func (r *openaiSSEReader) Close() error {
	r.done = true
	return r.body.Close()
}

type openaiChunkResponse struct {
	Choices []struct {
		Delta struct {
			Role      string               `json:"role"`
			Content   string               `json:"content"`
			ToolCalls []openaiToolUseChunk `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type openaiToolUseChunk struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (r *openaiSSEReader) parseChunk(data string) (provider.Event, error) {
	var chunk openaiChunkResponse
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil, fmt.Errorf("parse chunk: %w", err)
	}

	if len(chunk.Choices) == 0 {
		return nil, io.EOF
	}

	choice := chunk.Choices[0]

	// Check for role (signals message start)
	if choice.Delta.Role != "" {
		return &provider.MessageStartEvent{
			Role: choice.Delta.Role,
		}, nil
	}

	// Check for content delta
	if choice.Delta.Content != "" {
		return &provider.MessageDeltaEvent{
			Content: choice.Delta.Content,
		}, nil
	}

	// Check for tool calls
	if len(choice.Delta.ToolCalls) > 0 {
		tc := choice.Delta.ToolCalls[0]
		// For simplicity, emit tool calls as they come
		// In a real implementation, you'd accumulate arguments and emit when complete
		if tc.Function.Arguments != "" {
			return &provider.ToolCallEvent{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: json.RawMessage(tc.Function.Arguments),
			}, nil
		}
	}

	// Check for finish reason
	if choice.FinishReason != nil {
		return &provider.MessageEndEvent{
			StopReason: *choice.FinishReason,
		}, nil
	}

	// Empty delta, skip
	return r.NextEvent(context.Background())
}
