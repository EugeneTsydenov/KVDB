# Project Vision: Distributed Key-Value Database in Go

## 1. Project Overview
The project is a distributed Key-Value database designed primarily for cache storage. The system is inspired by Redis and will be implemented from scratch using the Go programming language.

The main purpose of the project is to gain practical experience in database systems, in-memory storage, networking, concurrency, performance optimization, fault tolerance, and distributed systems.

The system is designed as a scalable cluster of database nodes capable of handling a large number of concurrent clients while distributing data across multiple nodes.

## 2. Problem
Redis is a widely used Key-Value storage system, but implementing a similar system from scratch provides a deeper understanding of how high-performance distributed storage systems work internally.

The project aims to create a lightweight and extensible Key-Value database that can store data in memory, process requests efficiently, support expiration of stored values, execute atomic operations, and distribute data across multiple nodes.

The database is primarily intended for caching and temporary data storage, where low latency, high throughput, scalability, and fast failure recovery are important.

## 3. Target Scale
* **Cluster size:** up to approximately 300 database nodes.
* **Node capacity:** up to approximately 10,000 concurrently active clients per node. Since the system uses UDP, clients do not require a persistent connection in the same way as TCP; the target refers to the number of concurrently active clients sending requests.
* **Scalability:** horizontal scaling by adding nodes without requiring full dataset replication across every node.
* **Evaluation:** target scale is an architectural goal rather than a strict performance benchmark; actual metrics will be measured experimentally.

## 4. Key Features
* **SET** - store a value by key.
* **GET** - retrieve a value by key.
* **SET NX** - store a value only if the key does not already exist.
* **DEL** - delete a key.
* **TTL** - set and retrieve the lifetime of a key.
* Automatic removal of expired keys.
* Listing stored keys and filtering using regular expressions.
* Transactions for atomic execution of multiple operations.
* Lua scripts for atomic modification of data.
* Distributed operation across multiple database nodes.
* Request routing to the node responsible for the requested data.
* Sharding of the key space.
* Replication of shards for fault tolerance.
* Failure detection and automatic failover.
* Recovery of node state after failures.
* Rebalancing of shards when nodes are added or removed.

## 5. Data Distribution
The system will use sharding rather than full replication. Each key will be mapped to a logical shard using a hashing algorithm. The cluster will be divided into a fixed number of logical hash slots, and each slot will be assigned to a shard group.

**Data routing scheme:**
key "user:123" -> hash(key) -> slot 47 -> shard group -> primary: Node_A, replica: Node_B, replica: Node_C

A node will store only the keys belonging to the shards assigned to it rather than the complete dataset. The cluster will use a replication factor of approximately 3 (one primary and two replicas per shard).

**Key benefits of this architecture:**
* Cluster capacity grows with the number of nodes.
* Writes are propagated only to replicas of the affected shard.
* Node failure does not require communication with every node in the cluster.
* Adding or removing nodes requires redistribution of only a subset of the shards.

## 6. System Architecture
The main architectural layers are:
* **Client Layer:** clients send requests to the cluster.
* **Request Routing Layer:** calculates key hash, determines hash slot, identifies responsible shard group, and routes directly to the node.
* **Storage Layer:** stores and processes only keys belonging to assigned shards.
* **Replication Layer:** propagates updates from primary to replicas.
* **Membership and Failure Detection Layer:** gossip/SWIM-style failure detection maintains node state information.
* **Rebalancing and Recovery Layer:** migrates shards between nodes and restores state from replicas.

## 7. Networking
UDP will be used as the primary transport protocol for client and inter-node communication to avoid maintaining large numbers of persistent TCP connections.

**Application-level reliability mechanisms will include:**
* Request identifiers and response identifiers.
* Sequence numbers.
* Retry mechanisms and timeouts.
* Duplicate request detection.
* Request acknowledgements where required.
* Message size limits and operation batching.

## 8. Consistency and Replication
Each shard will have one primary node and a configurable number of replicas. Writes propagate from Client -> Primary -> Replicas.

Failure handling includes node membership tracking, health/failure detection, shard ownership tracking, primary failure detection, replica promotion, recovery synchronization, shard rebalancing, and state restoration.

## 9. TTL and Expiration
The database will support key expiration using TTL values based on a combination of:
* **Lazy expiration:** check expiration time when a key is accessed.
* **Active expiration:** periodically scan a subset of keys and remove expired entries.

## 10. Transactions and Lua Scripts
Transactions ensure atomic execution of groups of commands without interference. Lua scripts provide programmable atomic operations executed directly on the node responsible for the key/shard.

## 11. Optimization Decisions
* **Efficient Concurrency:** worker pools, sharded locks, lock striping, minimizing global locks, reducing goroutine allocation.
* **Memory Efficiency:** compact in-memory structures and minimal metadata overhead.
* **Request Batching:** batching multiple operations into single UDP packets.
* **Hash Slot Routing:** fixed logical hash slots (e.g. 4096) allowing dynamic shard ownership changes.
* **Efficient Expiration:** active scanning on subsets of keys.
* **Minimized Network Traffic:** direct request routing without full-mesh communication.
* **Rebalancing:** incremental slot migration when adding/removing nodes.

## 12. Technology
Go, UDP networking, in-memory data structures, hash-based sharding, replication, gossip/SWIM-style membership, Lua scripting, concurrent processing, custom binary wire protocol.

## 13. Project Goals & Expected Result
Build a working distributed Key-Value database in Go with cache features (TTL), atomic operations, Lua
