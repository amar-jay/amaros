package msgs

import (
	"encoding/json"
	"reflect"
	"time"
)

type AMAROS_MSG interface{}
type Message struct {
	AMAROS_MSG
	Data string `json:"data" msgpack:"data"`
}

type Header struct {
	AMAROS_MSG
	Seq     uint32
	Stamp   time.Time
	FrameId string
}

// TopicMetadata advertises topic semantics from the node that owns the topic.
// Nodes publish this on /topic.metadata so other nodes can discover how a topic is meant to be used.
type TopicMetadata struct {
	AMAROS_MSG
	Topic         string `json:"topic" msgpack:"topic"`
	Type          string `json:"type,omitempty" msgpack:"type,omitempty"`
	OwnerNode     string `json:"owner_node,omitempty" msgpack:"owner_node,omitempty"`
	Purpose       string `json:"purpose,omitempty" msgpack:"purpose,omitempty"`
	RequestTopic  string `json:"request_topic,omitempty" msgpack:"request_topic,omitempty"`
	ResponseTopic string `json:"response_topic,omitempty" msgpack:"response_topic,omitempty"`
	ResponseType  string `json:"response_type,omitempty" msgpack:"response_type,omitempty"`
}

// TODO: create a codegen for types. use this temporarily
func GetType(x interface{}) string {
	t := reflect.TypeOf(x)

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	result := map[string]string{}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		// get json tag
		name := f.Tag.Get("json")
		if f.Name == "AMAROS_MSG" {
			continue
		}

		// fallback to field name if no tag
		if name == "" {
			name = f.Name
		}

		result[name] = f.Type.Kind().String()
	}

	b, err := json.Marshal(result)
	if err != nil {
		return "unknown"
	}

	return string(b)
}
