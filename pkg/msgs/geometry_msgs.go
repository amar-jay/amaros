package msgs

// Vector3 represents a 3D vector.
type Vector3 struct {
	AMAROS_MSG
	X float64 `json:"x" msgpack:"x"`
	Y float64 `json:"y" msgpack:"y"`
	Z float64 `json:"z" msgpack:"z"`
}

type Quaternion struct {
	AMAROS_MSG
	X float64 `json:"x" msgpack:"x"`
	Y float64 `json:"y" msgpack:"y"`
	Z float64 `json:"z" msgpack:"z"`
	W float64 `json:"w" msgpack:"w"`
}

// Twist represents the velocity of a robot in free space broken into its linear and angular parts.
type Twist struct {
	AMAROS_MSG
	Linear  Vector3 `json:"linear" msgpack:"linear"`
	Angular Vector3 `json:"angular" msgpack:"angular"`
}

type Pose struct {
	AMAROS_MSG
	Position    Vector3    `json:"position" msgpack:"position"`
	Orientation Quaternion `json:"orientation" msgpack:"orientation"`
}

type Transform struct {
	AMAROS_MSG
	Translation Vector3    `json:"translation" msgpack:"translation"`
	Rotation    Quaternion `json:"rotation" msgpack:"rotation"`
}
