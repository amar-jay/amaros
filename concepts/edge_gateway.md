The current architecture used in Robotics to relay messages across machines usually relay on a dedicated server in the cloud.
However, this architecture doesn't really scale. This is prominent especially in systems where majority ~90% of data relayed
are low bandwidth data. This is inefficient. Also, noticing the growing evolution of edge rendering in the web, a better design 
for low bandwidth telemetry relying on edge networks as telemetry pipes.

## Core concept

Devices maintain a single outbound local WebSocket to the nearest edge POP of Cloudflare. That connection stays alive and acts like a lightweight telemetry pipe. At the edge, a Worker acts as a gateway: authentication, rate limiting, required transformations, and routing. From there messages are forwarded to a backend message fabric such as Zenoh or another event bus.

Since workers are stateless, information is coordinated across the message bus, essentially allowing synchronization across the 
network. This is analogous to data distribution service.

## Advantage

What makes this attractive is, most of the internet's global ingress runs on edge networks. They provide hundreds of entry points globally. Taking the advantages already surplanted by the exisiting edge systems, we can spread to even the most remote locations 
at edge speeds at almost no cost. Edge networks support bandwidths between 2Mbit/s to 100Gbit/s. : The de facto maximum transmission unit (MTU) on the internet is 1500 bytes.

Talking about cost, Edge computing basically costs nothing.	Telemetry messages are naturally small in size. (smaller than a webpage). Also taking into consideration the throughput is limited by the packet size. it is obviously natural. However the cost of edge computes are especially low.  

## Disadvantages
Despite the advantages that the edge network gives, it comes at the cost of stateless-ness. As stated earlier there is no need to maintain state so long as we can synchronize state across these networks. Also if necessary, Edge data stores could be utilized for temporary storage during latency issues. or misalignment across devices. Typically an edge data store is limited to 25MB, though this can be increased to 100MB upon request. This is more than enough for ~10,000 devices or more. 

## Architecture.
For building this, we can use cloudflare workers. 
Cloudflare workers are the best. That is more than any compliment I can give.
Zenoh as the underlying messaging system. We are currently rolling out our zenoh version, so most likely after that. 