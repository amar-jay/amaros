package main

import (
	"context"
	"fmt"
	"log"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"

	"github.com/amar-jay/amaros/internal/model"
	"github.com/amar-jay/amaros/internal/openrouter"
	"github.com/amar-jay/amaros/pkg/config"
	"github.com/amar-jay/amaros/pkg/memory"
	"github.com/amar-jay/amaros/pkg/msgs"
	"github.com/amar-jay/amaros/pkg/node"
	"github.com/amar-jay/amaros/pkg/topic"
)

const (
	defaultModel  = "openrouter/hunter-alpha"
	taskTopic     = "/llm.execute.task"
	questionTopic = "/telegram.question"
	responseTopic = "/telegram.response"
	resultTopic   = "/llm.execute.result"
	maxIterations = 50
	cmdTimeout    = 30 * time.Second
	llmTimeout    = 60 * time.Second
	responseWait  = 120 * time.Second
)

var (
	conf       *config.Config
	execNode   *node.Node
	task       = &msgs.ExecuteTask{}
	provider   model.Provider
	memManager *memory.Manager
)

func init() {
	conf = config.Get()

	apiKey := conf.OpenRouter.APIKey
	if apiKey == "" {
		log.Fatal("OpenRouter API key is not set. " +
			"Configure it via ~/.config/amaros/config.yaml (openrouter.api_key) " +
			"or the AMAROS_OPENROUTER_API_KEY environment variable.")
	}

	provider = openrouter.New(apiKey)

	execNode = node.Init("llm_execute_with_memory")
	execNode.DescribeTopics([]msgs.TopicMetadata{
		{
			Topic:         taskTopic,
			RequestTopic:  taskTopic,
			Type:          msgs.GetType(msgs.ExecuteTask{}),
			Purpose:       "task that should be handled by agent",
			ResponseTopic: resultTopic,
			ResponseType:  "{'task_id': string, 'success': bool, 'summary': string, 'output': string}",
		},
		{
			Topic:   resultTopic,
			Type:    msgs.GetType(msgs.ExecuteResult{}),
			Purpose: "answers returned by the llm_question_answer node to previously asked questions",
		},
	})
	execNode.OnShutdown(func() {
		fmt.Println("shutting down llm_execute_with_memory node")
		if memManager != nil {
			_ = memManager.Close()
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	memoryInstance, err := memory.NewManager(ctx, conf.Memory)
	if err != nil {
		log.Fatalf("failed to initialise memory manager: %v", err)
	}
	memManager = memoryInstance
}

func onTask(ctx topic.CallbackContext) {
	t := *task
	if t.Description == "" {
		ctx.Logger.Warn("received empty task, skipping")
		return
	}

	if t.TaskID == "" {
		id, err := gonanoid.New()
		if err != nil {
			ctx.Logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Warn("failed to generate task id")
			return
		}
		t.TaskID = id
	}

	ctx.Logger.WithFields(map[string]interface{}{
		"task_id":     t.TaskID,
		"description": t.Description,
	}).Info("received task, starting agentic loop with memory")

	go func(taskCopy msgs.ExecuteTask, topics []topic.Topic) {
		agent := NewAgent(provider, execNode, memManager, topics, maxIterations)
		agent.Run(&taskCopy)
	}(t, append([]topic.Topic(nil), ctx.Topics...))
}

// llm_execute_with_memory is an agentic node with persistent memory that receives task descriptions on
// /llm.execute.task and autonomously executes them by running shell commands in a loop.
func main() {
	fmt.Println("llm_execute_with_memory node started")
	fmt.Printf("  subscribed to: %s\n", taskTopic)
	fmt.Printf("  publishes to:  %s, %s\n", questionTopic, resultTopic)

	execNode.Callback(onTask)
	execNode.Subscribe(taskTopic, task)
}
