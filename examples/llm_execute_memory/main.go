package main

import (
	"fmt"
	"log"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"

	"github.com/amar-jay/amaros/internal/memory"
	"github.com/amar-jay/amaros/internal/model"
	"github.com/amar-jay/amaros/internal/model/openrouter"
	"github.com/amar-jay/amaros/pkg/config"
	"github.com/amar-jay/amaros/pkg/msgs"
	"github.com/amar-jay/amaros/pkg/node"
	"github.com/amar-jay/amaros/pkg/topic"
)

const (
	defaultModel  = "nvidia/nemotron-3-super-120b-a12b:free" //"openrouter/free"
	taskTopic     = "/llm.execute.task"
	questionTopic = "/telegram.question"
	resultTopic   = "/llm.execute.result"
	maxIterations = 200
	cmdTimeout    = 30 * time.Second
	llmTimeout    = 2 * time.Minute
	responseWait  = 5 * time.Minute
)

var (
	conf     *config.Config
	execNode *node.Node
	task     = &msgs.ExecuteTask{}
	provider model.Provider
	tm       *memory.TieredMemory
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

	// initialize memory
	var err error
	tm, err = memory.NewTieredMemory(conf.Memory.RootDir)
	if err != nil {
		log.Fatalf("failed to init memory: %v", err)
	}

	execNode = node.Init(node.NodeConfig{Name: "llm_execute_memory"})
	execNode.DescribeTopics([]msgs.TopicMetadata{
		{
			Topic:         taskTopic,
			RequestTopic:  taskTopic,
			Type:          msgs.GetType(msgs.ExecuteTask{}), //"{'task_id': string, 'description': string}",
			Purpose:       "task that should be handled by agent",
			ResponseTopic: resultTopic,
			ResponseType:  msgs.GetType(msgs.ExecuteResult{}), //"{'task_id': string, 'success': bool, 'summary': string, 'output': string}",
		},
		{
			Topic:   resultTopic,
			Type:    msgs.GetType(msgs.ExecuteResult{}), //"{'task_id': string, 'question_id': string, 'response': string}",
			Purpose: "answers returned by the llm_question_answer node to previously asked questions",
		},
	})
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
		agent := NewAgent(provider, execNode, topics, maxIterations, tm)
		agent.Run(&taskCopy)
	}(t, append([]topic.Topic(nil), ctx.Topics...))
}

// llm_execute_memory is an agentic node that receives task descriptions on
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
	fmt.Println("llm_execute_memory node started")
	fmt.Printf("  subscribed to: %s\n", taskTopic)
	fmt.Printf("  publishes to:  %s, %s\n", questionTopic, resultTopic)

	execNode.Callback(onTask)
	execNode.Subscribe(taskTopic, task)
}
