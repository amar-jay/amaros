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

type NodeConfig struct {
	Name string
	Tx   string
	Rx   string
}

func Init(c NodeConfig) *Node {

	n := &Node{
		Name: c.Name,
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sig
		if n.onshutdown != nil {
			n.onshutdown()
		}
	}()

	if c.Tx == "" {
		c.Tx = "localhost:11311"
	}
	if c.Rx == "" {
		c.Rx = "localhost:11312"
	}

	n.txConn = topic.DialServer(c.Tx)
	n.rxConn = topic.DialServer(c.Rx)
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

func (s *Node) Subscribe(_topic string, msg msgs.AMAROS_MSG) {
	topic.Subscribe(s.rxConn, s.txConn, _topic, msg, s.callback)
}

// SubscribeWithCallback subscribes to a topic using a specific callback function.
// This allows a node to handle multiple topic types with different handlers.
func (s *Node) SubscribeWithCallback(_topic string, msg msgs.AMAROS_MSG, callback func(topic.CallbackContext)) {
	topic.Subscribe(s.rxConn, s.txConn, _topic, msg, callback)
}
