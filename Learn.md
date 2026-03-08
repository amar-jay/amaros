This is a conceptual design document for a distributed agent orchestration system. WE ARE BUILDING IT A STEP AT A TIME.
THIS IS MEANT ONLY TO ALIGN THE GOAL BUT AI CODING AGENT SHOULD NEVER BE ALLOWED TO MAMARATHON THROUGH THIS PROJECT. IT SHOULD ONLY AND ONLY IMPLEMENT WHAT IT IS TOLD TO. THIS IS JUST FOR REMINDER. 
 The system is designed to be flexible, scalable, and modular, allowing for dynamic workflows and integration with various tools and services.
The project is more or less like a distributed graph of nodes that execute workflows.
A one singular organization can run multiple nodes. nodes aren't persistent, they can come and go. They report their capabilities and load metrics through a heartbeat mechanism. 
It is analogous to a ROS-style node graph, in ways similar to a ZeroMQ distributed system, with topics and events. Nodes can exist in different organizations(computer).
Nodes subscribe to topics and execute tasks based on their capabilities.
Events are like CRON jobs, but they can be triggered by external inputs or internal state changes.
Nodes are specialized and do one thing well, but they can be composed together to create complex workflows. They run in a loop, waiting for tasks to execute. They can also report their status and results back to the system.
Nodes can call external APIs, like models, databases, external APIs (weather, stocks, etc), messaging (whatsapp, telegram,..), search, and more. They can also call other nodes, but through message passing if within different organization, or direct function calls if within the same organization.
Nodes do only one thing, but they do it well.
This design allows for a highly modular and extensible system, where new capabilities can be added by simply creating new nodes and defining their interactions with existing nodes.

## Libraries (used but not limited to)
- Go
- zmq4 - for inter-node communication
- protobuf - for message serialization and comprehensive description of messages and events.
- viper - for configuration management (~/.config/copilot/)
- fiber - for human-facing node 
- sqlite - for state management and memory, registry in sqlite
- logrus - for logging
- cobra - for CLI tools

memory heirarchy:
in-memory -> file(readme)-> sqlite 

models:
model are called via openrouter api, which is a unified API for multiple models. This allows for flexibility in choosing different models based on the task at hand. Models can be used for various purposes.

