package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/amar-jay/amaros/pkg/msgs"
	"github.com/amar-jay/amaros/pkg/node"
	"github.com/amar-jay/amaros/pkg/topic"
)

const (
	requestTopic   = "/console.question"
	responseTopic  = "/console.response"
	requestTimeout = 60 * time.Second
)

var (
	llmNode  *node.Node
	question = &msgs.ExecuteQuestion{}
	result   = &msgs.ExecuteResult{}
	reader   = bufio.NewReader(os.Stdin)
)

func init() {

	llmNode = node.Init("console_messaging")
	llmNode.DescribeTopics([]msgs.TopicMetadata{
		{
			Topic:         requestTopic,
			Type:          msgs.GetType(msgs.ExecuteQuestion{}),
			Purpose:       "questions that require a human answer through the llm_question_answer node",
			ResponseTopic: responseTopic,
			ResponseType:  msgs.GetType(msgs.ExecuteResponse{}),
		},
		{
			Topic:   responseTopic,
			Type:    msgs.GetType(msgs.ExecuteResponse{}),
			Purpose: "answers returned by the llm_question_answer node to previously asked questions",
		},
		{
			Topic:   "/llm.execute.result",
			Type:    msgs.GetType(msgs.ExecuteResult{}),
			Purpose: "task results sent back to the requester",
		},
	})
	llmNode.OnShutdown(func() {
		fmt.Println("shutting down console_messaging node")
	})
}

func onRequest(ctx topic.CallbackContext) {
	// Copy the current request to avoid sharing the global pointer with future callbacks.
	req := *question
	if req.Question == "" {
		ctx.Logger.Warn("received empty question, skipping")
		return
	}

	// respond to the question through CLI
	ctx.Logger.WithFields(map[string]interface{}{
		"task_id":     req.TaskID,
		"question_id": req.QuestionID,
		"question":    req.Question,
	}).Info("Received question")
	fmt.Printf("Question: %s\n", req.Question)

	fmt.Print("Enter your answer: ")
	answer, err := reader.ReadString('\n')
	if err != nil {
		ctx.Logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("failed to read answer")
		return
	}
	answer = strings.TrimSpace(answer)
	fmt.Println("Answered")

	response := msgs.ExecuteResponse{
		TaskID:     req.TaskID,
		QuestionID: req.QuestionID,
		Response:   answer,
	}

	llmNode.Publish(responseTopic, response)
}

func onResult(ctx topic.CallbackContext) {
	res := *result

	status := "Failed"
	if res.Success {
		status = "Success"
	}

	fmt.Printf("\n--- Task Result [%s] ---\nStatus: %s\nSummary: %s\n", res.TaskID, status, res.Summary)
	if res.Output != "" {
		fmt.Printf("Output:\n%s\n", res.Output)
	}
	fmt.Println("-----------------------")
}

func main() {
	fmt.Printf("console_messaging node started\n")
	fmt.Printf("  subscribed to: %s\n", requestTopic)
	fmt.Printf("  subscribed to: /llm.execute.result\n")
	fmt.Printf("  publishing to: %s\n", responseTopic)

	llmNode.Callback(onRequest)
	llmNode.Subscribe(requestTopic, question)

	llmNode.Callback(onResult)
	llmNode.Subscribe("/llm.execute.result", result)

	// Keep the process running
	select {}
}
