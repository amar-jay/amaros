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
	requestTopic   = "/llm.execute.question"
	responseTopic  = "/llm.execute.response"
	requestTimeout = 60 * time.Second
)

var (
	llmNode  *node.Node
	question = &msgs.ExecuteQuestion{}
	reader   = bufio.NewReader(os.Stdin)
)

func init() {

	llmNode = node.Init("llm_question_answer")
	llmNode.DescribeTopics([]msgs.TopicMetadata{
		{
			Topic:         requestTopic,
			Type:          "*msgs.ExecuteQuestion",
			Purpose:       "questions that require a human answer through the llm_question_answer node",
			ResponseTopic: responseTopic,
			ResponseType:  "*msgs.ExecuteResponse",
		},
		{
			Topic:   responseTopic,
			Type:    "*msgs.ExecuteResponse",
			Purpose: "answers returned by the llm_question_answer node to previously asked questions",
		},
	})
	llmNode.OnShutdown(func() {
		fmt.Println("shutting down llm_question_answer node")
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

func main() {
	fmt.Printf("llm_question_answer node started\n")
	fmt.Printf("  subscribed to: %s\n", requestTopic)
	fmt.Printf("  publishing to: %s\n", responseTopic)

	llmNode.Callback(onRequest)
	llmNode.Subscribe(requestTopic, question)
}
