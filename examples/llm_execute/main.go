package main

import (
	"fmt"
	"log"
	"time"
	"github.com/matoous/go-nanoid/v2"

	"github.com/amar-jay/amaros/internal/config"
	"github.com/amar-jay/amaros/internal/model"
	"github.com/amar-jay/amaros/internal/openrouter"
	"github.com/amar-jay/amaros/pkg/msgs"
	"github.com/amar-jay/amaros/pkg/node"
	"github.com/amar-jay/amaros/pkg/topic"
)

const (
	defaultModel  = "openrouter/hunter-alpha"
	taskTopic     = "/llm.execute.task"
	questionTopic = "/llm.execute.question"
	responseTopic = "/llm.execute.response"
	resultTopic   = "/llm.execute.result"
	maxIterations = 50
	cmdTimeout    = 30 * time.Second
	llmTimeout    = 60 * time.Second
	responseWait  = 120 * time.Second
)

var (
	conf     *config.Config
	execNode *node.Node
	task     = &msgs.ExecuteTask{}
	provider model.Provider
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

	execNode = node.Init("llm_execute")
	execNode.OnShutdown(func() {
		fmt.Println("shutting down llm_execute node")
	})
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
	}).Info("received task, starting agentic loop")

	go func(taskCopy msgs.ExecuteTask, topics []topic.Topic) {
		agent := NewAgent(provider, execNode, topics, maxIterations)
		agent.Run(&taskCopy)
	}(t, append([]topic.Topic(nil), ctx.Topics...))
}

// llm_execute is an agentic node that receives task descriptions on
// /llm.execute.task and autonomously executes them by running shell
// commands in a loop. It uses an LLM to decide the next action at
// each step.
//
// Topics:
//
//	subscribes: /llm.execute.task     — incoming task descriptions
//	publishes:  /llm.execute.question — questions for the user
//	subscribes: /llm.execute.response — user answers (temporary)
//	publishes:  /llm.execute.result   — final task result
func main() {
	fmt.Println("llm_execute node started")
	fmt.Printf("  subscribed to: %s\n", taskTopic)
	fmt.Printf("  publishes to:  %s, %s\n", questionTopic, resultTopic)

	execNode.Callback(onTask)
	execNode.Subscribe(taskTopic, task)
}
