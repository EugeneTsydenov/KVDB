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

* **Go** (version `1.25` or higher)
* **Task** (Build tool alternative to `make`) — [Installation Guide](https://taskfile.dev/installation/)
* **golangci-lint** — [Installation Guide](https://golangci-lint.run/welcome/install/)

## 🛠 Development Commands (Taskfile)

We use `Taskfile.yml` to streamline building, testing, linting, and formatting tasks.

### Available Tasks:

| Command | Description |
| :--- | :--- |
| `task` | Show help and list all available tasks |
| `task test` | Run unit tests with race condition detection (`-race`) |
| `task test:short` | Run quick tests without race detection |
| `task test:cover` | Run tests and generate an HTML coverage report |
| `task bench` | Run Go benchmarks |
| `task lint` | Run `golangci-lint` for code quality checks |
| `task fmt` | Format code using `gofmt` / `golangci-lint fmt` |
| `task deps` | Download and tidy up Go module dependencies |
| `task clean` | Remove temporary build files and coverage reports |

## 📁 Structure

```
cmd/        # server and cli entry points
internal/   # storage, protocol, expiration, txn, script, server
docs/       # documentation
```

More details: [`adept/wiki/VISION.md`](adept/wiki/VISION.md)

## 📜 License

MIT
