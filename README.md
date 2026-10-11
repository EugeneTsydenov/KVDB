<div align="center">

# 🗄️ Distributed In-Memory Key-Value Storage

**Distributed in-memory Key-Value database for caching, written from scratch in Go**

![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)
![Status](https://img.shields.io/badge/status-work%20in%20progress-orange)
![License](https://img.shields.io/badge/license-MIT-blue)

</div>

## 📖 About

A Redis-inspired Key-Value database built for learning: in-memory storage, UDP networking, concurrency, sharding, replication and failure detection.

## ✨ Features

- `SET`, `GET`, `SET NX`, `DEL`
- TTL and automatic removal of expired keys
- Key listing with regex filtering
- Transactions
- Lua scripts
- Sharding, replication, failover and rebalancing

## 🚀 Prerequisites

To build and run this project locally, ensure you have the following installed:

* **Go** (version `1.22` or higher)
* **Task** (Build tool alternative to `make`) — [Installation Guide](https://taskfile.dev/installation/)
* **golangci-lint** — [Installation Guide](https://golangci-lint.run/welcome/install/)

## 🚀 Quick Start

```bash
git clone https://github.com/<your-username>/<repo-name>.git
cd <repo-name>

go run ./cmd/server   # start a node
go run ./cmd/cli      # start the client
```

## 📁 Structure

```
cmd/        # server and cli entry points
internal/   # storage, protocol, expiration, txn, script, server
docs/       # documentation
```

More details: [`adept/wiki/VISION.md`](adept/wiki/VISION.md)

## 📜 License

MIT
