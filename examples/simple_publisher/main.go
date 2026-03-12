package main

import (
	"github.com/amar-jay/amaros/pkg/msgs"
	"github.com/amar-jay/amaros/pkg/node"
)

func main() {
	node := node.Init("simple_node")
	node.OnShutdown(func() {
		println("shutting down node")
	})

	// msg := msgs.Quaternion{
	// 	X: 0.1,
	// 	Y: 0.2,
	// 	Z: 0.3,
	// 	W: 0.4,
	// }
	// msg := msgs.LLMRequest{
	// 	Model:  "openrouter/free",
	// 	Prompt: "What is the capital of France?",
	// }
	msg := msgs.ExecuteTask{
		Description: "do you have a music topic? if not, what topics do you have access to.",
	}
	//msg := "Hello World"
	node.Publish("/llm.execute.task", msg)
}
