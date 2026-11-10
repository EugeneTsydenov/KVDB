# Software Requirements Specification

## KVDB — Distributed Key-Value Database

10 November 2026

Authors: Tsydenov Evgeniy, Tikhonov Artem, Koidenko Semen, Stecenko Danil

## 1. Introduction

### 1.1. Project idea

KVDB is a Key-Value database written in Go. It is mainly designed to store cached and temporary data. A client uses a key to save, read, or delete a value. Keys may also have an expiration time.

The final system is planned to work on several servers. Each server will store part of the data, and replicas will help the system continue working if a server fails.

The project is inspired by Redis, but the storage engine and network protocol are developed by the team.

### 1.2. Purpose

The goal is to build a working database and learn how in-memory storage, concurrency, network protocols, transactions, and distributed systems work.

The database is intended for applications that need fast access to temporary values. It is not intended to replace a permanent relational database.

### 1.3. Terms

- **Key:** A name used to find stored data.
- **Value:** The data stored under a key.
- **TTL:** The time left before a key expires.
- **Node:** One running database server.
- **Shard:** A part of the database's data.
- **Slot:** A logical group of keys used to distribute data.
- **Primary:** A node that accepts changes for a shard.
- **Replica:** A node that keeps a copy of a shard.
- **Failover:** Choosing a new primary when the old one fails.

## 2. Users and system parts

The main user is a developer whose application needs a cache. The application sends commands to KVDB and reads the results.

A developer or operator starts database nodes and checks their state. The project does not require a web interface.

The system will have these main parts:

- An in-memory storage engine for keys, values, and expiration times.
- A UDP server that accepts requests and sends responses.
- A client program or library that uses the binary protocol.
- Transaction and Lua script support for atomic changes.
- Cluster components for routing, replication, failure detection, and recovery.

## 3. Functional requirements

### 3.1. Basic commands

The database must support these commands:

- `PING` checks whether the server responds.
- `SET` stores a value under a key. If the key already exists, its value and TTL are replaced.
- `GET` returns the value of a key. If the key does not exist or has expired, the result is `NOT_FOUND`.
- `SETNX` stores a value only if the key does not exist or has expired. If the key exists, the result is `NOT_SET`.
- `DEL` deletes one key and returns `1` if the key was removed or `0` if the key was missing or expired.

A value is stored as bytes. Commands that change the same key at the same time must not corrupt the stored data.

### 3.2. Expiration

A client must be able to set a TTL when it stores a value. TTL is given in milliseconds. A TTL of `0` in `SET` or `SETNX` means that the key does not expire.

The `TTL` command returns the remaining lifetime. It returns `-1` when a key has no expiration time and `NOT_FOUND` when the key does not exist or has expired. A remaining time shorter than one millisecond is returned as `0`.

The `EXPIRE` command changes the lifetime of an existing key. Its TTL must be greater than zero.

Expired keys must act like missing keys, even if they have not yet been physically removed. The system must remove expired keys during access and also clean them up in the background.

### 3.3. Key search

The `KEYS` command must find keys using a regular expression. KVDB uses the RE2 regular expression syntax available in Go. Expired keys must not appear in the results.

The server must return matching keys in lexicographic order. In the current protocol version, all results must fit into one response packet. If they do not fit, the server returns `TOO_LARGE` instead of sending a partial list.

The packet already contains `cursor` and `limit` fields, but pagination is not supported yet. Both request fields must be `0` in this version.

### 3.4. Transactions

The database must support a way to run several commands as one atomic operation. Other commands must not observe a partly completed transaction.

For example, a client may need to delete several keys and create new keys as one operation. The transaction format, error handling, and support for operations on different shards still need to be designed.

### 3.5. Lua scripts

Clients must be able to run Lua scripts for changes that require several steps. A script must execute atomically relative to other operations on the same data.

The team still needs to define how scripts are sent, which commands scripts can use, how execution time is limited, and whether a script may access more than one shard.

### 3.6. Distributed storage

The final database must run as a cluster of nodes. It will use hashing to map each key to a logical slot, then assign slots to shard groups.

A node stores only the shards assigned to it. The system must route a request to the node responsible for its key.

Each shard group is planned to have one primary and two replicas by default. The primary sends changes to its replicas. The number of replicas should be configurable.

The cluster must detect failed nodes, select a replacement primary when possible, and restore missing copies of data. It must also move slots between nodes when servers are added or removed.

The exact rules for confirming writes, selecting a new primary, and resolving network splits have not been chosen yet.

## 4. Main use cases

### 4.1. Store and read a value

Actor: Client application.

1. The client sends `SET` with a key, value, and optional expiration time.
2. The server stores the value and returns `OK`.
3. The client sends `GET` for the same key.
4. The server returns the value, unless the key was deleted or expired.

### 4.2. Create a value only once

Actor: Client application.

1. The client sends `SETNX`.
2. The server checks whether the key exists and is not expired.
3. If the key is missing, it stores the new value and returns `OK`.
4. Otherwise, it returns `NOT_SET` and keeps the old value.

### 4.3. Work with temporary data

Actor: Client application.

1. The client stores a value with a TTL.
2. The client can request the remaining TTL or change it with `EXPIRE`.
3. After the expiration time, `GET` returns `NOT_FOUND`.
4. The server removes the expired entry during access or background cleanup.

### 4.4. Find stored keys

Actor: Client application.

1. The client sends `KEYS` with a regular expression.
2. The server finds matching, non-expired keys.
3. The server returns the sorted list if it fits into one packet.
4. Otherwise, the server returns `TOO_LARGE`.

### 4.5. Recover after a node failure

Actors: Cluster nodes.

1. A primary node stops responding.
2. Other nodes detect the failure.
3. The cluster selects a suitable replica as the new primary when possible.
4. Requests are routed to the new primary.
5. The cluster restores missing replicas after recovery.

This is a planned scenario. The election and recovery rules are still under design.

## 5. Network protocol

KVDB uses a custom binary protocol over UDP. One datagram contains one request or one response. All multi-byte numbers use big-endian order.

### 5.1. Packet header

Each packet starts with a 16-byte header:

- `magic` — 2 bytes, value `0x4B56`.
- `version` — 1 byte, currently `0x01`.
- `opcode` — 1 byte, command type.
- `status` — 1 byte, result status.
- `flags` — 1 byte, request or response flag.
- `payload_len` — 2 bytes, payload length.
- `request_id` — 8 bytes, used to match a response to a request.

The lowest bit of `flags` is `0` for requests and `1` for responses. Other flag bits must be zero. The request status is always zero. A response keeps the request's command code and request ID.

### 5.2. Packet limits

- Maximum packet size: 1400 bytes.
- Header size: 16 bytes.
- Maximum payload size: 1384 bytes.
- Maximum key size: 256 bytes.
- Maximum value size: 1024 bytes.
- Maximum regular expression size: 256 bytes.
- Maximum error message size: 256 bytes.
- Maximum TTL: 9,223,372,036,854 milliseconds.

Byte arrays have a two-byte length field followed by the data. Text strings use the same format and are encoded in UTF-8.

### 5.3. Supported command codes

- `0x01` — `PING`.
- `0x02` — `GET`.
- `0x03` — `SET`.
- `0x04` — `SETNX`.
- `0x05` — `DEL`.
- `0x06` — `TTL`.
- `0x07` — `EXPIRE`.
- `0x08` — `KEYS`.

Commands for transactions, Lua scripts, and cluster management are not part of this protocol version yet.

### 5.4. Response statuses

The protocol defines `OK`, `NOT_FOUND`, `NOT_SET`, `BAD_REQUEST`, `UNKNOWN_OPCODE`, `UNSUPPORTED_VERSION`, `TOO_LARGE`, `INVALID_ARGUMENT`, and `INTERNAL_ERROR`.

The decoder must reject invalid packets safely. A broken packet must not cause a panic. Packets with an invalid size, magic value, or payload length are dropped without a response. Other errors use the status codes described in `docs/protocol.md`.

UDP does not guarantee delivery or order. The planned client and server behavior includes timeouts, retries, and detection of repeated requests. These rules still need a separate design. In particular, repeating a write must not lead to an unexpected second execution.

## 6. Non-functional requirements

### 6.1. Technology

KVDB is written in Go and uses in-memory storage. Clients communicate through UDP and the project's binary protocol. The code must support concurrent requests safely.

### 6.2. Performance

The long-term design goal is a cluster of about 300 nodes and about 10,000 active UDP clients per node. These numbers are targets for the architecture, not measured results.

The database should have low response time and handle many requests at once. The team will measure latency, throughput, memory use, and CPU use during testing. Exact acceptance limits have not been set.

### 6.3. Reliability

Malformed packets must not crash the protocol decoder. The storage engine must not lose consistency because of concurrent operations.

In cluster mode, the system should keep serving available data when a node fails and a suitable replica can take over. The accepted recovery time and possible loss of recent writes must be defined before cluster testing.

### 6.4. Resource usage

The implementation should avoid unnecessary memory copies, long global locks, and excessive network traffic. Background expiration should process a limited number of keys at a time rather than blocking the storage engine for a full scan.

## 7. Current development state

This section describes the code in the `develop` branch reviewed on 10 November 2026. It is not a list of finished product features.

The following parts already have Go implementations:

- A thread-safe in-memory store with `Set`, `Get`, `SetNX`, `Del`, `TTL`, `Expire`, and regex-based `Keys` operations.
- Lazy expiration during storage operations.
- A background expiration worker that periodically deletes expired entries in batches.
- Request and response encoding and decoding for the eight command codes in protocol version 1.
- Packet constants, size checks, status codes, and binary read/write helpers.
- Unit tests for the storage engine, TTL behavior, and concurrent storage access.

The storage's `Keys` method currently returns matches without sorting them. Sorting is required by the protocol description and must be done before a server response is sent.

The server and CLI entry-point directories do not yet contain running programs. Therefore, the repository does not yet provide a complete client-to-server UDP workflow.

The transaction, Lua script, and server directories are placeholders. The distributed routing, replication, failure detection, failover, and rebalancing features described above are also not present as working cluster components in the reviewed branch.

The README lists the planned features, but this list should not be treated as evidence that they are all implemented.

## 8. Testing

The project should be checked with unit, integration, and performance tests.

Storage tests must cover normal operations, missing keys, replacement of values, TTL expiration, and concurrent access. Key search tests must cover valid and invalid patterns, expired keys, sorting, and large results.

Protocol tests must check header fields, byte order, request IDs, packet limits, invalid payloads, and status codes. Integration tests will be needed when the UDP server and client are available.

Later cluster tests must cover request routing, replication, node failure, primary replacement, recovery, and rebalancing. Transactions and Lua scripts must be tested for atomic execution when they are implemented.

Performance tests must report the hardware, number of nodes and clients, request sizes, latency, and throughput so that results can be compared.

## 9. Decisions still needed

Before the final version of this specification, the team needs to agree on several points:

1. Will transactions and Lua scripts be part of the first release or added later?
2. Can one transaction or script change keys on different shards?
3. When is a replicated write considered successful: after the primary, a majority, or all replicas confirm it?
4. What consistency guarantees are required during node failures and network splits?
5. What hash function and number of logical slots will be used?
6. How will nodes select a primary and avoid two primaries for one shard?
7. What are the required response time, throughput, recovery time, and availability targets?
8. What rules will the UDP client use for timeouts, retries, and duplicate requests?
9. May all cached data be lost if every replica fails, or is disk persistence required?
10. Will the project require client authentication, encryption, or cluster administration commands?

These questions do not change the basic storage and protocol requirements, but they are important for the final distributed version.
