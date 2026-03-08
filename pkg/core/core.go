package core

import (
	"bufio"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"sync"

	ilog "github.com/amar-jay/amaros/internal/logger"
	t "github.com/amar-jay/amaros/pkg/topic"
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

func (r *Core) HandleConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		// Read the incoming message from the client
		message, err := reader.ReadString('\n')
		if err != nil {
			// most likely the client disconnected, so we close the connection, else user must reconnect
			break
		}

		message = strings.TrimSpace(message)
		tokens := strings.SplitN(message, " ", 2)

		var command, topic string
		if len(tokens) == 2 {
			command, topic = tokens[0], tokens[1]
		} else if len(tokens) == 1 {
			command = tokens[0]
		} else {
			conn.Write([]byte("Invalid command\n"))
			println("Invalid command", message)
			continue
		}

		_type := "unknown"
		switch command {
		case "SUBSCRIBE":
			r.Subscribe(topic, _type, conn)
		case "UNSUBSCRIBE":
			r.Unsubscribe(topic, conn)
		case "PUBLISH":
			r.Publish(topic, conn)
		case "STATUS":
			r.Status(topic, conn)
		case "LIST":
			r.List(conn)
		default:
			conn.Write([]byte("Unknown command\n"))
		}
	}
}

func (r *Core) Subscribe(topic_type string, _type string, conn net.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	parts := strings.SplitAfterN(topic_type, " ", 2)
	if len(parts) < 2 {
		conn.Write([]byte("Invalid publish format. Use: SUBSCRIBE <topic> <type>\n"))
		return
	}
	topic, _type := parts[0], parts[1]
	_type = strings.TrimSpace(_type)
	topic = strings.TrimSpace(topic)

	r.Subscribers[topic] = append(r.Subscribers[topic], conn)
	if r.Types[topic] != "" && _type != r.Types[topic] {
		conn.Write([]byte("Invalid message type format. Use: SUBSCRIBE <topic> " + r.Types[topic] + "\n"))
		return
	}
	if r.Types[topic] == "" {
		r.Types[topic] = _type
	}

	// since a topic can have multiple subscribers, we keep track of all the subscribers in a slice.
	//fmt.Println("Client", conn.RemoteAddr(), "subscribed to topic", topic, "type", _type)
	r.logger.WithFields(map[string]interface{}{
		"topic": topic,
		"type":  _type,
	}).Debug("New subscription")
	// there is no need to send a message to the client that they have subscribed successfully
	msg, _ := json.Marshal(map[string]string{
		"message": "subscribed successfully",
	})
	conn.Write([]byte(topic + " " + string(msg) + "\n"))
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

func (r *Core) Publish(topic_message string, conn net.Conn) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	parts := strings.SplitAfterN(topic_message, " ", 2)
	if len(parts) < 2 {
		conn.Write([]byte("Invalid publish format. Use: PUBLISH <topic> <message>\n"))
	} else {
		topic, message := parts[0], parts[1]
		message = strings.TrimSpace(message)
		topic = strings.TrimSpace(topic)

		for _, conn := range r.Subscribers[topic] {
			conn.Write([]byte(topic + " " + string(message) + "\n"))
			r.logger.WithFields(map[string]interface{}{
				"topic":   topic,
				"message": message,
			}).Debug("Publishing message")
		}
	}
}

func (r *Core) Status(topic string, conn net.Conn) {
	status := t.Status{Subscribers: map[string]int{topic: len(r.Subscribers[topic])}, Type: r.Types[topic]}
	for t, conns := range r.Subscribers {
		status.Subscribers[t] = len(conns)
	}
	st, err := json.Marshal(status)
	if err != nil {
		r.logger.Error("Error marshalling status")
		return
	}

	conn.Write([]byte(string(st) + "\n"))

}

func (r *Core) List(conn net.Conn) {
	topics := make([]t.Topic, 0, len(r.Subscribers))
	for _t := range r.Subscribers {
		topics = append(topics, t.Topic{Name: _t}) // that is to assume list does not need to know the type
	}
	st, err := json.Marshal(topics)
	if err != nil {
		r.logger.Error("Error marshalling status")
		return
	}

	conn.Write([]byte(string(st) + "\n"))
}
