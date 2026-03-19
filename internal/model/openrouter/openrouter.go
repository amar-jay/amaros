package openrouter

// This package is responsible for handling the OpenRouter API interactions.
// It provides functions to send requests to the OpenRouter API and process the responses for different LLM/VLA models.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	. "github.com/amar-jay/amaros/internal/model"
)

const baseURL = "https://openrouter.ai/api/v1/chat/completions"

// Client implements Provider for the OpenRouter API.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// New creates a new OpenRouter client with the given API key.
func New(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: http.DefaultClient,
	}
}

func (c *Client) Name() string {
	return "openrouter"
}

// openrouter request/response shapes
type orRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"` // custom JSON via Message.MarshalJSON
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

type orResponse struct {
	ID      string     `json:"id"`
	Model   string     `json:"model"`
	Choices []orChoice `json:"choices"`
	Usage   Usage      `json:"usage"`
}

type orChoice struct {
	Message Message `json:"message"`
}

// Complete sends a completion request to OpenRouter and returns the response.
func (c *Client) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	body, err := json.Marshal(orRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var orResp orResponse
	if err := json.Unmarshal(respBody, &orResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	content := ""
	if len(orResp.Choices) > 0 {
		content = orResp.Choices[0].Message.Content
	}

	return &CompletionResponse{
		ID:      orResp.ID,
		Model:   orResp.Model,
		Content: content,
		Usage:   orResp.Usage,
	}, nil
}
