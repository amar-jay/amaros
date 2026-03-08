package topic

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	ilog "github.com/amar-jay/amaros/internal/logger"
	"github.com/amar-jay/amaros/pkg/msgs"
	msgpack "github.com/shamaton/msgpack/v2"
)

type Topic struct {
	Name    string      `json:"name" msgpack:"name"`
	Type    string      `json:"type,omitempty" msgpack:"type,omitempty"`
	Message interface{} `json:"message,omitempty" msgpack:"message,omitempty"`
}

type Status struct {
	Subscribers map[string]int `json:"subscribers" msgpack:"subscribers"`
	Type        string         `json:"type" msgpack:"type"`
}

var topics = make([]Topic, 0)
var logger *ilog.Logger

func init() {
	logger = ilog.New()
	logger.SetLevel("debug")
}

func DialServer(address string) net.Conn {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		os.Exit(1)
	}
	return conn
}

func writeEnvelope(conn net.Conn, env msgs.Envelope) error {
	data, err := msgpack.Marshal(env)
	if err != nil {
		return err
	}
	_, err = conn.Write(data)
	return err
}

func handleUnsubscribe(conn net.Conn, topic string) {
	if err := writeEnvelope(conn, msgs.Envelope{Cmd: msgs.CmdUnsubscribe, Topic: topic}); err != nil {
		logger.Error("Failed to send UNSUBSCRIBE:", err)
	}
	logger.WithFields(map[string]interface{}{
		"topic": topic,
	}).Debug("Unsubscribed from topic")
}

type CallbackContext struct {
	Logger *ilog.Logger // not well written right, never mind it should be left so.
	Params string
	// add more fields as needed
}

func handleSubscribe(conn net.Conn, topic string, msg msgs.ROS_MSG, callback func(CallbackContext)) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		handleUnsubscribe(conn, topic)
		os.Exit(1)
	}()

	for {
		var env msgs.Envelope
		if err := msgpack.UnmarshalRead(conn, &env); err != nil {
			logger.Error("Server disconnected:", err)
			return
		}

		if env.Cmd == msgs.RespError {
			logger.Error("Server error:", env.Err)
			continue
		}

		if env.Cmd == msgs.RespOK {
			// subscribe acknowledgment, no action needed
			continue
		}

		if env.Cmd != msgs.RespMessage {
			logger.Error("Unexpected message type:", env.Cmd)
			continue
		}

		if len(env.Payload) == 0 {
			continue
		}

		if err := msgpack.Unmarshal(env.Payload, msg); err != nil {
			logger.Error("Unmarshal msgpack error:", err)
			continue
		}

		// put it in type msg
		if logger == nil {
			println("No logger present")
		}

		if env.Topic == topic && callback != nil {
			callback(CallbackContext{Logger: logger, Params: ""})
		}
	}
}

func handleStatus(conn net.Conn, topic string) {
	var env msgs.Envelope
	if err := msgpack.UnmarshalRead(conn, &env); err != nil {
		logger.Error("Server disconnected.", err)
		return
	}

	if env.Cmd == msgs.RespError {
		logger.Error("Server error:", env.Err)
		return
	}

	if env.Cmd != msgs.RespStatus {
		logger.Error("Unexpected response type:", env.Cmd)
		return
	}

	var msg Status
	if err := msgpack.Unmarshal(env.Payload, &msg); err != nil {
		logger.Error("Unmarshal msgpack error:", err)
		return
	}
	logger.WithFields(map[string]interface{}{
		"topic":       topic,
		"subscribers": msg.Subscribers[topic],
		"type":        msg.Type,
	}).Debug("Topic status")
}

func handleList(conn net.Conn) {
	var env msgs.Envelope
	if err := msgpack.UnmarshalRead(conn, &env); err != nil {
		logger.Error("Server disconnected.", err)
		return
	}

	if env.Cmd == msgs.RespError {
		logger.Error("Server error:", env.Err)
		return
	}

	if env.Cmd != msgs.RespList {
		logger.Error("Unexpected response type:", env.Cmd)
		return
	}

	if err := msgpack.Unmarshal(env.Payload, &topics); err != nil {
		logger.Error("Unmarshal msgpack error:", env.Cmd, err)
		return
	}
	for _, topic := range topics {
		logger.Debug("Topic: ", topic.Name)
	}
}

func Publish(conn net.Conn, topic string, message msgs.ROS_MSG) {
	payload, err := msgpack.Marshal(message)
	if err != nil {
		logger.Error("invalid message type. unable to marshal message:", err)
		return
	}

	if err := writeEnvelope(conn, msgs.Envelope{Cmd: msgs.CmdPublish, Topic: topic, Payload: payload}); err != nil {
		logger.Error("Failed to send PUBLISH:", err)
	}
}

func Subscribe(conn net.Conn, topic string, msg msgs.ROS_MSG, callback func(CallbackContext)) {
	topicType := fmt.Sprintf("%T", msg)
	if err := writeEnvelope(conn, msgs.Envelope{Cmd: msgs.CmdSubscribe, Topic: topic, TopicType: topicType}); err != nil {
		logger.Error("Failed to send SUBSCRIBE:", err)
		return
	}
	handleSubscribe(conn, topic, msg, callback)
}

func List(conn net.Conn) {
	if err := writeEnvelope(conn, msgs.Envelope{Cmd: msgs.CmdList}); err != nil {
		logger.Error("Failed to send LIST:", err)
		return
	}
	handleList(conn)
}

func SubscribeStatus(conn net.Conn, topic string) {
	if err := writeEnvelope(conn, msgs.Envelope{Cmd: msgs.CmdStatus, Topic: topic}); err != nil {
		logger.Error("Failed to send STATUS:", err)
		return
	}
	handleStatus(conn, topic)
}
