package main

import (
	"fmt"
	"time"

	"github.com/amar-jay/amaros/pkg/msgs"
	"github.com/amar-jay/amaros/pkg/node"
	"github.com/amar-jay/amaros/pkg/topic"
)

var t *msgs.Quaternion
var n *node.Node

func init() {
	t = &msgs.Quaternion{}
	n = node.Init("node_proxy")
	n.OnShutdown(func() {
		println("shutting down node proxy")
	})
}

func callback(ctx topic.CallbackContext) {
	fmt.Printf("received: %v\n", t)

	// Transform the message by doubling each component
	transformed := msgs.Quaternion{
		X: t.X * 2,
		Y: t.Y * 2,
		Z: t.Z * 2,
		W: t.W * 2,
	}

	fmt.Printf("publishing transformed: %v at %s\n", transformed, time.Now().String())
	n.Publish("/chatter_transformed", transformed)
}

// node_proxy subscribes to "/chatter", doubles the quaternion values,
// and republishes the transformed message on "/chatter_transformed".
func main() {

	n.Callback(callback)
	n.Subscribe("/chatter", t)
}
