package main

import (
	"fmt"
	"time"

	"github.com/amar-jay/amaros/pkg/msgs"
	"github.com/amar-jay/amaros/pkg/node"
	"github.com/amar-jay/amaros/pkg/topic"
)

func main() {
	node := node.Init("simple_node")
	node.OnShutdown(func() {
		println("shutting down node")
	})

	/*NOTE:
	     * does not work with &new(msgs.Quaternion) / var t *msgs.Quaternion / var t msgs.Quaternion.
			 * It works only with the expression msg := &msgs.Quaternion{}.
			 * Still trying to figure out why.
			 * This is the only way to make it work.
			 * Other types like string, int, float32, float64, bool, etc. work fine with their respective expressions.
			 * Tried other ways but does not have type safety nearly as good as this.
	*/

	t := &msgs.ExecuteResult{}
	node.Callback(func(ctx topic.CallbackContext) {
		fmt.Printf("%v\n", t)
		// if there is a question, answer it through cli
		if t.Success {
			ctx.Logger.WithFields(map[string]interface{}{
				"summary": t.Summary,
			}).Info(t.Output)
		} else {
			ctx.Logger.WithFields(map[string]interface{}{
				"summary": t.Summary,
			}).Error("Received error")
			fmt.Printf("Error : %s\n", t.Output)
		}
		println(time.Now().String(), "callback called")
	})
	node.Subscribe("/llm.execute.result", t)
}
