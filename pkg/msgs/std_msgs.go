package msgs

import "time"

type ROS_MSG interface{}
type String struct {
	ROS_MSG
	Str string `json:"str" msgpack:"str"`
}

type Int struct {
	ROS_MSG
	Int int `json:"int" msgpack:"int"`
}

type Float struct {
	ROS_MSG
	Float float64 `json:"float" msgpack:"float"`
}

type Bool struct {
	ROS_MSG
	Bool bool `json:"bool" msgpack:"bool"`
}

type ColorRGBA struct {
	ROS_MSG
	R float32 `json:"R" msgpack:"R"`
	G float32 `json:"G" msgpack:"G"`
	B float32 `json:"B" msgpack:"B"`
	A float32 `json:"A" msgpack:"A"`
}

type ColorRGB struct {
	ROS_MSG
	R float32  `json:"R" msgpack:"R"`
	G float32  `json:"G" msgpack:"G"`
	B float32  `json:"B" msgpack:"B"`
}

type Header struct {
	ROS_MSG
	Seq     uint32
	Stamp   time.Time
	FrameId string
}

// TopicMetadata advertises topic semantics from the node that owns the topic.
// Nodes publish this on /topic.metadata so other nodes can discover how a topic is meant to be used.
type TopicMetadata struct {
	ROS_MSG
	Topic         string `json:"topic" msgpack:"topic"`
	Type          string `json:"type,omitempty" msgpack:"type,omitempty"`
	OwnerNode     string `json:"owner_node,omitempty" msgpack:"owner_node,omitempty"`
	Purpose       string `json:"purpose,omitempty" msgpack:"purpose,omitempty"`
	RequestTopic  string `json:"request_topic,omitempty" msgpack:"request_topic,omitempty"`
	ResponseTopic string `json:"response_topic,omitempty" msgpack:"response_topic,omitempty"`
	ResponseType  string `json:"response_type,omitempty" msgpack:"response_type,omitempty"`
}
