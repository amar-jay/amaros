package node

import (
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/amar-jay/amaros/pkg/msgs"
	"github.com/amar-jay/amaros/pkg/topic"
)

type Node struct {
	Name       string
	onshutdown func()
	callback   func(topic.CallbackContext) // to listen for messages
	txConn     net.Conn                    // connection for publishing (TX)
	rxConn     net.Conn                    // connection for subscribing (RX)
	txMu       sync.Mutex
}

func Init(name string) *Node {

	n := &Node{
		Name: name,
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sig
		if n.onshutdown != nil {
			n.onshutdown()
		}
	}()

	n.txConn = topic.DialServer("localhost:11311")
	n.rxConn = topic.DialServer("localhost:11312")
	return n
}

func (n *Node) OnShutdown(f func()) {
	n.onshutdown = f
}

func (n *Node) Callback(f func(topic.CallbackContext)) {
	n.callback = f
}
func (p *Node) Publish(_topic string, msg interface{}) {
	p.txMu.Lock()
	defer p.txMu.Unlock()
	topic.Publish(p.txConn, _topic, msg)
}

func (n *Node) DescribeTopic(meta msgs.TopicMetadata) {
	if meta.OwnerNode == "" {
		meta.OwnerNode = n.Name
	}
	n.txMu.Lock()
	defer n.txMu.Unlock()
	topic.Publish(n.txConn, topic.MetadataTopicName, meta)
}

func (n *Node) DescribeTopics(metadata []msgs.TopicMetadata) {
	for _, meta := range metadata {
		n.DescribeTopic(meta)
	}
}

func (s *Node) Subscribe(_topic string, msg msgs.ROS_MSG) {
	topic.Subscribe(s.rxConn, _topic, msg, s.callback)
}

// SubscribeWithCallback subscribes to a topic using a specific callback function.
// This allows a node to handle multiple topic types with different handlers.
func (s *Node) SubscribeWithCallback(_topic string, msg msgs.ROS_MSG, callback func(topic.CallbackContext)) {
	topic.Subscribe(s.rxConn, _topic, msg, callback)
}
