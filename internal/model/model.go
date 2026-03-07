package model

import (
	"context"
	"encoding/json"
)

// Role represents the role of a message in a conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// ContentType identifies the kind of content in a message part.
type ContentType string

const (
	ContentText     ContentType = "text"
	ContentImageURL ContentType = "image_url"
)

// ContentPart represents one piece of a multimodal message.
type ContentPart struct {
	Type     ContentType `json:"type"`
	Text     string      `json:"text,omitempty"`
	ImageURL *ImageURL   `json:"image_url,omitempty"`
}

// ImageURL holds the URL (or base64 data URI) for an image part.
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "auto", "low", "high"
}

// Message represents a single message in a conversation.
// For text-only messages, set Content. For multimodal messages, use Parts instead.
type Message struct {
	Role    Role          `json:"role"`
	Content string        `json:"-"`
	Parts   []ContentPart `json:"-"`
}

// MarshalJSON serialises Message to the OpenAI/OpenRouter format.
// If Parts is non-empty, content is an array of parts; otherwise it is a plain string.
func (m Message) MarshalJSON() ([]byte, error) {
	if len(m.Parts) > 0 {
		return json.Marshal(struct {
			Role    Role          `json:"role"`
			Content []ContentPart `json:"content"`
		}{Role: m.Role, Content: m.Parts})
	}
	return json.Marshal(struct {
		Role    Role   `json:"role"`
		Content string `json:"content"`
	}{Role: m.Role, Content: m.Content})
}

// UnmarshalJSON handles both string and array content fields.
func (m *Message) UnmarshalJSON(data []byte) error {
	var raw struct {
		Role    Role            `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	m.Role = raw.Role

	// Try string first
	var s string
	if err := json.Unmarshal(raw.Content, &s); err == nil {
		m.Content = s
		return nil
	}

	// Otherwise, array of parts
	var parts []ContentPart
	if err := json.Unmarshal(raw.Content, &parts); err != nil {
		return err
	}
	m.Parts = parts
	return nil
}

// TextPart is a convenience constructor for a text content part.
func TextPart(text string) ContentPart {
	return ContentPart{Type: ContentText, Text: text}
}

// ImagePart is a convenience constructor for an image content part.
// url can be a regular URL or a base64 data URI ("data:image/png;base64,...").
func ImagePart(url string, detail string) ContentPart {
	p := ContentPart{Type: ContentImageURL, ImageURL: &ImageURL{URL: url}}
	if detail != "" {
		p.ImageURL.Detail = detail
	}
	return p
}

// CompletionRequest is the input to a model completion call.
type CompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// CompletionResponse is the output from a model completion call.
type CompletionResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Content string `json:"content"`
	Usage   Usage  `json:"usage"`
}

// Usage tracks token consumption.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Provider is the interface that model backends must implement.
type Provider interface {
	// Complete sends a completion request and returns the response.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

	// Name returns the provider name (e.g. "openrouter").
	Name() string
}
