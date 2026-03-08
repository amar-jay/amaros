package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/amar-jay/amaros/internal/config"
	"github.com/amar-jay/amaros/internal/model"
	"github.com/amar-jay/amaros/internal/openrouter"
	"github.com/amar-jay/amaros/pkg/msgs"
	"github.com/amar-jay/amaros/pkg/node"
	"github.com/amar-jay/amaros/pkg/topic"
)

const (
	defaultModel   = "openrouter/free"
	requestTopic   = "/llm.request"
	responseTopic  = "/llm.response"
	requestTimeout = 60 * time.Second
)

var (
	conf     *config.Config
	llmNode  *node.Node
	llmReq   = &msgs.LLMRequest{}
	provider model.Provider
)

const sysPromptPrefix = `
`

func init() {
	conf = config.Get()

	apiKey := conf.OpenRouter.APIKey
	if apiKey == "" {
		log.Fatal("OpenRouter API key is not set. " +
			"Configure it via ~/.config/amaros/config.yaml (openrouter.api_key) " +
			"or the AMAROS_OPENROUTER_API_KEY environment variable.")
	}

	provider = openrouter.New(apiKey)

	llmNode = node.Init("llm_inference")
	llmNode.OnShutdown(func() {
		fmt.Println("shutting down llm_inference node")
	})
}

func onRequest(ctx topic.CallbackContext) {
	// Copy the current request to avoid sharing the global pointer with future callbacks.
	req := *llmReq
	if req.Prompt == "" {
		ctx.Logger.Warn("received empty prompt, skipping")
		return
	}

	modelName := req.Model
	if modelName == "" {
		modelName = defaultModel
	}

	messages := make([]model.Message, 0, 2)
	if req.SystemPrompt != "" {
		messages = append(messages, model.Message{
			Role:    model.RoleSystem,
			Content: req.SystemPrompt,
		})
	}
	messages = append(messages, model.Message{
		Role:    model.RoleUser,
		Content: req.Prompt,
	})

	compReq := model.CompletionRequest{
		Model:       modelName,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	}

	ctx.Logger.WithFields(map[string]interface{}{
		"model":  modelName,
		"prompt": req.Prompt,
	}).Info("sending LLM request")

	callCtx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	resp, err := provider.Complete(callCtx, compReq)
	if err != nil {
		ctx.Logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("LLM request failed")
		return
	}

	ctx.Logger.WithFields(map[string]interface{}{
		"model":             resp.Model,
		"prompt_tokens":     resp.Usage.PromptTokens,
		"completion_tokens": resp.Usage.CompletionTokens,
	}).Info("LLM response: %s", resp.Content)

	llmNode.Publish(responseTopic, &msgs.LLMResponse{
		Content:          resp.Content,
		Model:            resp.Model,
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
	})
}

// llm_inference is a node that bridges the amaros pub/sub system with the
// OpenRouter LLM API. It subscribes to /llm/request and publishes responses
// to /llm/response.
//
// Configuration (via ~/.config/amaros/config.yaml or environment variables):
//
//	openrouter:
//	  api_key: "<your OpenRouter API key>"
//
// Or set AMAROS_OPENROUTER_API_KEY in the environment.
func main() {
	fmt.Printf("llm_inference node started\n")
	fmt.Printf("  subscribed to: %s\n", requestTopic)
	fmt.Printf("  publishing to: %s\n", responseTopic)

	llmNode.Callback(onRequest)
	llmNode.Subscribe(requestTopic, llmReq)
}
