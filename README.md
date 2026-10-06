# Distributed In-Memory Key-Value Storage

A high-performance In-Memory Key-Value Database written in Go, featuring a custom UDP protocol, transactions, TTL support, and an embedded Lua interpreter.

---

## 📌 Overview

This project is a high-load, in-memory data store. During the first phase, development is focused on building a single-node core, handling network I/O over UDP, ensuring concurrency safety, and supporting transactional operations.

For a detailed technical vision, architectural goals, and long-term roadmap, please refer to the [VISION.md](adept/wiki/VISION.md) document.

---

## 🚀 Prerequisites

To build and run this project locally, ensure you have the following installed:

* **Go** (version `1.22` or higher)
* **Task** (Build tool alternative to `make`) — [Installation Guide](https://taskfile.dev/installation/)
* **golangci-lint** — [Installation Guide](https://golangci-lint.run/welcome/install/)

---

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
