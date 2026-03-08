package registry

// node registry
// located in ~/.config/amaros/registry
// contains a number of nodes, can be delpoyed there from a local or remote registry (first local)
// a node can run a task, given a topic runs an event, returns a response,

// there are model nodes; these call an model LLM model (be it a chat interface LLM or a code agent LLM or a terminal LLM)
// terminal LLM runs a series of request and response until the goal is accomplished. primarily in bash terminal
// code agent LLM is generally a writer, it writes to files, and then executres and return response, it may be a single file run or a series of files
// a chat LLM, is a human facing llm agent, request and response to a human user. it has a code agent and a terminal LLM within to allow these functionality whilst chating

// there are also sensor nodes; these nodes, read sensor data connected to the device, you can request read via sensor topic /sensor.imu, not limited to sensors to also includes actuators /sensor.actuator
// there are chat nodes; these are nodes that are integrated to a telegram API, it regularly reads chat from the API, it has a chat LLM within with all its sub nodes.
// mavlink modes; mavlink nodes integrates with the mavlink api to communicate with drones/rovers. using the topic actions and states can be queried. it contains a number of inner sensor nodes
// this is a broader description. a step by step first, first let's build the registry(folders and local and remote service), then we will think about the rest.

// FIRST STEP! CREATE A REGISTRY, IT HAS
