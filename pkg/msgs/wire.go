package msgs

const (
	// Client → Server command types
	CmdSubscribe   uint8 = 0x01
	CmdUnsubscribe uint8 = 0x02
	CmdPublish     uint8 = 0x03
	CmdStatus      uint8 = 0x04
	CmdList        uint8 = 0x05

	// Server → Client response types
	RespOK      uint8 = 0x10
	RespError   uint8 = 0x11
	RespMessage uint8 = 0x12
	RespStatus  uint8 = 0x13
	RespList    uint8 = 0x14
)

// Envelope is the wire protocol message format used for all communication
// between nodes and the core server. Every message in both directions is
// encoded as a msgpack-serialized Envelope.
type Envelope struct {
	Cmd       uint8  `msgpack:"c"`
	Topic     string `msgpack:"t,omitempty"`
	TopicType string `msgpack:"tt,omitempty"`
	Payload   []byte `msgpack:"p,omitempty"`
	Err       string `msgpack:"e,omitempty"`
}
