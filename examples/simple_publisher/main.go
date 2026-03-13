package main

import (
	"flag"

	"github.com/amar-jay/amaros/pkg/msgs"
	"github.com/amar-jay/amaros/pkg/node"
)

func main() {
	// Take question from cli
	question := flag.String("question", "", "Question to publish")
	flag.Parse()
	if *question == "" {
		panic("Question Description is required")
	}

	node := node.Init("simple_node")
	node.OnShutdown(func() {
		println("shutting down node")
	})

	// msg := msgs.LLMRequest{
	// 	Model:  "openrouter/free",
	// 	Prompt: "What is the capital of France?",
	// }
	msg := msgs.ExecuteTask{
		Description: *question,
	}
	//msg := "Hello World"
	node.Publish("/llm.execute.task", msg)
}
