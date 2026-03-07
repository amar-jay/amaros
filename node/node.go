package node

import (
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/amar-jay/amaros/msgs"
	"github.com/amar-jay/amaros/topic"
)

type Node struct {
	Name       string
	onshutdown func()
	callback   func() // to listen for messages
	conn       net.Conn
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
			println("shutting down node: ", n.Name)
			n.onshutdown()
		}
	}()

	n.conn = topic.DialServer("localhost:11311")
	return n
}

func (n *Node) OnShutdown(f func()) {
	n.onshutdown = f
}

func (n *Node) Callback(f func()) {
	n.callback = f
}
func (p *Node) Publish(_topic string, msg interface{}) {
	topic.Publish(p.conn, _topic, msg)
}

func (s *Node) Subscribe(_topic string, msg msgs.ROS_MSG) {
	topic.Subscribe(s.conn, _topic, msg, s.callback)
}
