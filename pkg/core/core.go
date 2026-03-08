package core

import (
	"net"
	"strconv"
	"sync"

	ilog "github.com/amar-jay/amaros/internal/logger"
	"github.com/amar-jay/amaros/pkg/msgs"
	t "github.com/amar-jay/amaros/pkg/topic"
	msgpack "github.com/shamaton/msgpack/v2"
)

// hmmm! A synchronous map will be more useful here. However, this is just a simple example
type Core struct {
	mu          sync.RWMutex
	Subscribers map[string][]net.Conn // map of topic to subscribers (connections)
	Types       map[string]string     // ros topic types map
	logger      *ilog.Logger
}

func NewCore() *Core {
	logger := ilog.New()
	return &Core{
		mu:          sync.RWMutex{},
		Subscribers: make(map[string][]net.Conn),
		Types:       make(map[string]string),
		logger:      logger,
	}
}

func (r *Core) LogLevel(level string) {
	if r.logger != nil {
		if level == "warn" || level == "error" || level == "debug" || level == "info" {
			r.logger.SetLevel(level)
		} else {
			r.logger.Warnf("invalid log level %q, defaulting to info", level)
			r.logger.SetLevel("info")
		}
	}
}

func (r *Core) Listen(host string, txPort int, rxPort int) {
	txAddr := host + ":" + strconv.Itoa(txPort)
	rxAddr := host + ":" + strconv.Itoa(rxPort)

	txLn, err := net.Listen("tcp", txAddr)
	if err != nil {
		r.logger.Error("Error starting TX server:", err)
		return
	}

	rxLn, err := net.Listen("tcp", rxAddr)
	if err != nil {
		txLn.Close()
		r.logger.Error("Error starting RX server:", err)
		return
	}

	r.logger.Printf("roscore TX (publish) listening on tcp://%s/\n", txAddr)
	r.logger.Printf("roscore RX (subscribe) listening on tcp://%s/\n", rxAddr)

	go func() {
		defer rxLn.Close()
		for {
			conn, err := rxLn.Accept()
			if err != nil {
				r.logger.Error("Error accepting RX connection:", err)
				continue
			}
			go r.HandleConn(conn)
		}
	}()

	defer txLn.Close()
	for {
		conn, err := txLn.Accept()
		if err != nil {
			r.logger.Error("Error accepting TX connection:", err)
			continue
		}
		go r.HandleConn(conn)
	}
}

func writeEnvelope(conn net.Conn, env msgs.Envelope) error {
	data, err := msgpack.Marshal(env)
	if err != nil {
		return err
	}
	_, err = conn.Write(data)
	return err
}

func (r *Core) HandleConn(conn net.Conn) {
	defer conn.Close()
	for {
		var env msgs.Envelope
		if err := msgpack.UnmarshalRead(conn, &env); err != nil {
			// most likely the client disconnected, so we close the connection
			break
		}

		switch env.Cmd {
		case msgs.CmdSubscribe:
			r.Subscribe(env.Topic, env.TopicType, conn)
		case msgs.CmdUnsubscribe:
			r.Unsubscribe(env.Topic, conn)
		case msgs.CmdPublish:
			r.Publish(env.Topic, env.Payload, conn)
		case msgs.CmdStatus:
			r.Status(env.Topic, conn)
		case msgs.CmdList:
			r.List(conn)
		default:
			writeEnvelope(conn, msgs.Envelope{Cmd: msgs.RespError, Err: "unknown command"})
		}
	}
}

func (r *Core) Subscribe(topic string, topicType string, conn net.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if topic == "" {
		writeEnvelope(conn, msgs.Envelope{Cmd: msgs.RespError, Err: "invalid subscribe: missing topic"})
		return
	}

	r.Subscribers[topic] = append(r.Subscribers[topic], conn)
	if r.Types[topic] != "" && topicType != r.Types[topic] {
		writeEnvelope(conn, msgs.Envelope{Cmd: msgs.RespError, Err: "invalid message type, expected " + r.Types[topic]})
		return
	}
	if r.Types[topic] == "" {
		r.Types[topic] = topicType
	}

	r.logger.WithFields(map[string]interface{}{
		"topic": topic,
		"type":  topicType,
	}).Debug("New subscription")
	writeEnvelope(conn, msgs.Envelope{Cmd: msgs.RespOK, Topic: topic})
}

func (r *Core) Unsubscribe(topic string, conn net.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, c := range r.Subscribers[topic] {
		if c == conn {
			r.Subscribers[topic] = append(r.Subscribers[topic][:i], r.Subscribers[topic][i+1:]...)
			r.logger.WithFields(map[string]interface{}{
				"topic": topic,
			}).Debug("Client unsubscribed from topic")
			break
		}
	}

	// if empty delete
	if len(r.Subscribers[topic]) == 0 {
		delete(r.Subscribers, topic)
	}

	delete(r.Types, topic)
	// no need to send a message to the client that they have unsubscribed successfully
}

func (r *Core) Publish(topic string, payload []byte, conn net.Conn) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if topic == "" {
		writeEnvelope(conn, msgs.Envelope{Cmd: msgs.RespError, Err: "invalid publish: missing topic"})
		return
	}

	fwd := msgs.Envelope{Cmd: msgs.RespMessage, Topic: topic, Payload: payload}
	for _, sub := range r.Subscribers[topic] {
		if err := writeEnvelope(sub, fwd); err != nil {
			r.logger.WithFields(map[string]interface{}{
				"topic": topic,
			}).Error("Error forwarding message to subscriber:", err)
		} else {
			r.logger.WithFields(map[string]interface{}{
				"topic": topic,
			}).Debug("Publishing message")
		}
	}
}

func (r *Core) Status(topic string, conn net.Conn) {
	status := t.Status{Subscribers: map[string]int{topic: len(r.Subscribers[topic])}, Type: r.Types[topic]}
	for topicName, conns := range r.Subscribers {
		status.Subscribers[topicName] = len(conns)
	}
	payload, err := msgpack.Marshal(status)
	if err != nil {
		r.logger.Error("Error marshalling status")
		return
	}

	writeEnvelope(conn, msgs.Envelope{Cmd: msgs.RespStatus, Payload: payload})
}

func (r *Core) List(conn net.Conn) {
	topicList := make([]t.Topic, 0, len(r.Subscribers))
	for topicName := range r.Subscribers {
		topicList = append(topicList, t.Topic{Name: topicName}) // that is to assume list does not need to know the type
	}
	payload, err := msgpack.Marshal(topicList)
	if err != nil {
		r.logger.Error("Error marshalling topics list")
		return
	}

	writeEnvelope(conn, msgs.Envelope{Cmd: msgs.RespList, Payload: payload})
}
