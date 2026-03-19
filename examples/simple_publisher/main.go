package main

import (
	"flag"

	"github.com/amar-jay/amaros/internal/model"
	"github.com/amar-jay/amaros/pkg/node"
)

func main() {
	// Take question from cli
	name := flag.String("name", "simple_node", "Name of node")
	m := flag.String("model", "openrouter/free", "Model to use for completion")
	question := flag.String("question", "", "Question to publish")
	flag.Parse()

	if *question == "" {
		panic("Question Description is required")
	}

	node := node.Init(*name)
	node.OnShutdown(func() {
		println("shutting down node")
	})

	msg := model.CompletionRequest{
		Model: *m,
		Messages: []model.Message{
			{
				Role:    model.RoleUser,
				Content: "" + *question,
			},
		},
	}

	node.Publish("/llm.execute", msg)
}
