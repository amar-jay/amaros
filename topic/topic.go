package topic

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	ilog "github.com/amar-jay/amaros/internal/logger"
	"github.com/amar-jay/amaros/msgs"
)

type Topic struct {
	Name    string `json:"name"`
	Type    string
	Message interface{} `json:"message,omitempty"`
}

type Status struct {
	Subscribers map[string]int `json:"subscribers"`
	Type        string         `json:"type"`
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

func handleUnsubscribe(conn net.Conn, topic string) {
	fmt.Fprintf(conn, "UNSUBSCRIBE %s\n", topic)
	logger.WithFields(map[string]interface{}{
		"topic": topic,
	}).Debug("Unsubscribed from topic")
}

type CallbackContext struct {
	Logger *ilog.Logger // not well written right, never mind it should be left so.
	// add more fields as needed
}

func handleSubscribe(conn net.Conn, topic string, msg msgs.ROS_MSG, callback func(CallbackContext)) {
	reader := bufio.NewReader(conn)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		handleUnsubscribe(conn, topic)
		os.Exit(1)
	}()

	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			logger.Error("Server disconnected:", err)
			return
		}
		message = strings.TrimSpace(message)
		m := strings.SplitAfterN(message, " ", 2)
		if len(m) < 2 {
			logger.Error("Invalid message from server:", m, len(m))
		}

		_topic, message := m[0], m[1]
		message = strings.TrimSpace(message)
		if len(message) == 0 {
			continue
		}

		err = json.Unmarshal([]byte(message), &msg)

		if err != nil {
			logger.Error("Unmarshal json error", err)
			continue
		}

		// put it in type msg
		//logger.Debug("message received type: ", reflect.TypeOf(&msg).String())

		if strings.TrimSpace(_topic) == topic && callback != nil {
			callback(CallbackContext{Logger: logger})
		}
	}
}

func handleStatus(conn net.Conn, topic string) {
	reader := bufio.NewReader(conn)
	message, err := reader.ReadString('\n')
	if err != nil {
		logger.Error("Server disconnected.", err)
		return
	}
	message = strings.TrimSpace(message)
	if len(message) == 0 {
		return
	}

	var msg Status

	err = json.Unmarshal([]byte(message), &msg)
	if err != nil {
		logger.Error("Unmarshal json error", err)
		return
	}
	logger.WithFields(map[string]interface{}{
		"topic":       topic,
		"subscribers": msg.Subscribers[topic],
		"type":        msg.Type,
	}).Debug("Topic status")
}

func handleList(conn net.Conn) {
	reader := bufio.NewReader(conn)
	message, err := reader.ReadString('\n')
	if err != nil {
		logger.Error("Server disconnected.", err)
		return
	}
	message = strings.TrimSpace(message)
	if len(message) == 0 {
		return
	}

	err = json.Unmarshal([]byte(message), &topics)
	if err != nil {
		logger.Error("Unmarshal json error: ", message, "\n", err)
		return
	}
	for _, topic := range topics {
		logger.Debug("Topic: ", topic.Name)
	}
}

func Publish(conn net.Conn, topic string, message msgs.ROS_MSG) {
	msg, err := json.Marshal(message)
	if err != nil {
		logger.Error("invalid message type. unable to parse message")
	}

	// Send PUBLISH command to server
	fmt.Fprintf(conn, "PUBLISH %s %s\n", topic, msg)
}

func Subscribe(conn net.Conn, topic string, msg msgs.ROS_MSG, callback func(CallbackContext)) {
	fmt.Fprintf(conn, "SUBSCRIBE %s %T\n", topic, msg)
	handleSubscribe(conn, topic, msg, callback)
}

func List(conn net.Conn) {
	fmt.Fprintf(conn, "LIST\n")
	handleList(conn)
}

func SubscribeStatus(conn net.Conn, topic string) {
	fmt.Fprintf(conn, "STATUS %s\n", topic)
	handleStatus(conn, topic)
}
