# AMAROS
This is an agentic orchestrator system that provides a lightweight pub/sub messaging layer with a node registry/packaging system. It is designed to be subscriber-frist, meaning that for topics to exist, there must be at least one subscriber. This encourages a more dynamic and flexible architecture where nodes can come and go, and topics are created organically based on demand. Using this architecture, support for unix socket communication locally and TCP communication(multiplex) remotely, allowing for flexible scenarios.

Nodes can be installed from a remote registry, allowing for easy sharing and reuse of functionality. Or built locally.

Amongst the nodes built locally include a LLM execution node, which using agentic loops can execute complex tasks using a series of bash scripts, and a messaging node which can send messages to the console or Telegram.

> [!NOTE]
> During the development, I was not familiar with Zenoh. Apparently, Zenoh is a protocol that achieves everything AMAROS was set to achieve regarding the messaging layer. However, A Go port of Zenoh does not exist yet. Well, so goodbye to this project, but I will keep it here for posterity and as a learning experience. If you want to build something like this, I recommend looking into Zenoh and building on top of it instead of reinventing the wheel. 

---

By default it listens on TCP ports `11311` (publish/tx) and `11312` (subscribe/rx) / you can change it in place of UNIX sockets if you want.

---

Examples are located under the `examples/` folder and include:

- `examples/llm_execute/` — run LLM-powered tasks via a node executor.
- `examples/messaging/` — console and Telegram messaging nodes.
- `examples/simple_publisher/` and `examples/simple_subscriber/` — basic pub/sub demos.

Basic architecture overview:

- `pkg/core`: Core server logic (topic routing, subscriptions, metadata).
- `pkg/topic`: Client tooling for publishing, subscribing, and topic discovery.
- `pkg/registry`: Remote registry client + local store for node packages.
- `pkg/node`: Node runtime + execution helpers.
- `pkg/config`: Configuration handling.
