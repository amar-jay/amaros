package topic

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	ilog "github.com/amar-jay/amaros/internal/logger"
	"github.com/amar-jay/amaros/pkg/msgs"
	msgpack "github.com/shamaton/msgpack/v2"
)

type Topic struct {
	Name          string      `json:"name" msgpack:"name"`
	Type          string      `json:"type,omitempty" msgpack:"type,omitempty"`
	Subscribers   int         `json:"subscribers,omitempty" msgpack:"subscribers,omitempty"`
	OwnerNode     string      `json:"owner_node,omitempty" msgpack:"owner_node,omitempty"`
	Purpose       string      `json:"purpose,omitempty" msgpack:"purpose,omitempty"`
	RequestTopic  string      `json:"request_topic,omitempty" msgpack:"request_topic,omitempty"`
	ResponseTopic string      `json:"response_topic,omitempty" msgpack:"response_topic,omitempty"`
	ResponseType  string      `json:"response_type,omitempty" msgpack:"response_type,omitempty"`
	Message       interface{} `json:"message,omitempty" msgpack:"message,omitempty"`
}

const MetadataTopicName = "/topic.metadata"

type Status struct {
	Subscribers map[string]int `json:"subscribers" msgpack:"subscribers"`
	Type        string         `json:"type" msgpack:"type"`
}

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
	Topics []Topic /* available topics node can access */
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

		topics, err := FetchList(conn)
		if err != nil {
			logger.Error(err)
			return
		}

		if env.Topic == topic && callback != nil {
			callback(CallbackContext{Logger: logger, Params: "", Topics: topics})
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

func handleList(conn net.Conn) ([]Topic, error) {
	var env msgs.Envelope
	var topics = make([]Topic, 0)
	if err := msgpack.UnmarshalRead(conn, &env); err != nil {
		return nil, fmt.Errorf("Server disconnected. %s", err.Error())
	}

	if env.Cmd == msgs.RespError {
		return nil, fmt.Errorf("Server error: %s", env.Err)
	}

	if env.Cmd != msgs.RespList {
		return nil, fmt.Errorf("Unexpected response type: %d", env.Cmd)
	}

	if err := msgpack.Unmarshal(env.Payload, &topics); err != nil {
		logger.Error()
		return nil, fmt.Errorf("Unmarshal msgpack error: %d %s", env.Cmd, err.Error())
	}
	return topics, nil
}

func Publish(conn net.Conn, topic string, message msgs.ROS_MSG) {
	if err := Validate(topic); err != nil {
		logger.Error("Error: invalid topic ", topic)
		return
	}

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
	if err := Validate(topic); err != nil {
		logger.Error("Error: invalid topic ", topic)
		return
	}

	topicType := msgs.GetType(msg)
	if err := writeEnvelope(conn, msgs.Envelope{Cmd: msgs.CmdSubscribe, Topic: topic, TopicType: topicType}); err != nil {
		logger.Error("Failed to send SUBSCRIBE:", err)
		return
	}
	handleSubscribe(conn, topic, msg, callback)
}

func FetchList(conn net.Conn) ([]Topic, error) {
	if err := writeEnvelope(conn, msgs.Envelope{Cmd: msgs.CmdList}); err != nil {
		return nil, fmt.Errorf("failed to send LIST: %w", err)
	}
	return handleList(conn)
}

func List(conn net.Conn) {
	topics, err := FetchList(conn)
	if err != nil {
		logger.Error(err)
		return
	}
	for _, topic := range topics {
		logger.Debug("Topic: ", topic.Name)
	}
}

func SubscribeStatus(conn net.Conn, topic string) {

	if err := Validate(topic); err != nil {
		logger.Error("Error: invalid topic ", topic)
		return
	}

	if err := writeEnvelope(conn, msgs.Envelope{Cmd: msgs.CmdStatus, Topic: topic}); err != nil {
		logger.Error("Failed to send STATUS:", err)
		return
	}
	handleStatus(conn, topic)
}

// validTopicRe matches topic names of the form /namespace/name or /name.
// Each segment must start with a letter or underscore and contain only
// alphanumeric characters and underscores.
var validTopicRe = regexp.MustCompile(`^(/[a-zA-Z_][a-zA-Z0-9_.]*)+$`)

// Validate returns an error if name is not a valid RoboOS topic name.
// Valid names start with '/' and consist of one or more slash-separated
// segments, each beginning with a letter or underscore.
// Examples of valid names:   /sensor.imu, /robot.sensor.imu, /robot.arm.joint1
func Validate(name string) error {
	if name == "" {
		return fmt.Errorf("topic name must not be empty")
	}
	if !strings.HasPrefix(name, "/") {
		return fmt.Errorf("topic name %q must start with '/'", name)
	}
	if strings.HasSuffix(name, "/") {
		return fmt.Errorf("topic name %q must not end with '/'", name)
	}
	// shouldn't contain "/" after the first character, since that would indicate a segment with an empty name
	if strings.Contains(name[1:], "/") {
		return fmt.Errorf("topic name %q must not contain '/' after the first character", name)
	}
	// Reject names that contain consecutive '/' or '.' to avoid ambiguity in parsing.
	if strings.Contains(name, "//") || strings.Contains(name, "..") {
		return fmt.Errorf("topic name %q must not contain consecutive '/' or '.'", name)
	}
	if !validTopicRe.MatchString(name) {
		return fmt.Errorf("topic name %q is invalid: use /namespace/name format (alphanumeric + underscore segments)", name)
	}
	return nil
}
