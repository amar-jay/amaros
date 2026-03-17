The current architecture used in Robotics to relay messages across machines usually relay on a dedicated server in the cloud.
However, this architecture doesn't really scale. This is prominent especially in systems where majority ~90% of data relayed
are low bandwidth data. This is inefficient. Also, noticing the growing evolution of edge rendering in the web, a better design 
for low bandwidth telemetry relying on edge networks as telemetry pipes.

## Core concept

Devices maintain a single outbound local WebSocket to the nearest edge POP of Cloudflare. That connection stays alive and acts like a lightweight telemetry pipe. At the edge, a Worker acts as a gateway: authentication, rate limiting, required transformations, and routing. From there messages are forwarded to a backend message fabric such as Zenoh or another event bus.

Since workers are stateless, information is coordinated across the message bus, essentially allowing synchronization across the 
network. This is analogous to data distribution service.

## Advantages

- Global Ingress & Reach: By utilizing established edge networks, the system gains hundreds of global entry points. This allows robots in remote locations to achieve "local-loop" speeds.

- Elastic Bandwidth: Edge networks comfortably handle a massive range of throughput, from 2 Mbit/s up to 100 Gbit/s, while adhering to the standard internet  MTU of 1500 bytes.

- Cost Efficiency: Telemetry packets are significantly smaller than standard web pages. Because edge computing providers charge based on execution time and request volume rather than persistent "server-up" time, the cost per device is negligible compared to traditional cloud VMs.

## Challenges & Mitigations
The primary trade-off in this design is the stateless nature of edge workers. However, for telemetry, persistent state is often unnecessary if the underlying messaging layer (Zenoh) handles synchronization.

- Temporary Persistence: If synchronization issues or latency spikes occur, Edge Key-Value (KV) stores can be utilized for transient buffering.

- Capacity: With standard edge storage limits (25MB to 100MB), the architecture can easily manage state or caching for 10,000+ concurrent devices.

## Architecture
The proposed stack leverages Cloudflare Workers for the ingress and logic layer due to their industry-leading cold-start performance and global footprint. Zenoh serves as the underlying protocol, providing the high-throughput, low-overhead multipoint communication required for modern robotics.
