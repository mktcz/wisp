# Wisp

**A Lighter, Simpler Go Development Runner**

Wisp is a minimalist, file-watching and live-reloading command-line tool designed for modern Go development. It automatically rebuilds and restarts your application(s) when file changes are detected, with support for running multiple services concurrently.

## Features

- **Concurrent App Runner**: Run multiple applications simultaneously from a single configuration
- **Intelligent File Watching**: Debounced file change detection prevents build spam
- **Reliable Process Control**: Strict lifecycle management (kill → wait → build → run)
- **TOML Configuration**: Clean, simple configuration format
- **Targeted Commands**: Run all apps or specific ones
- **Graceful Shutdown**: Proper signal handling and cleanup
- **Smart Delays**: Configurable delays between operations to ensure reliability

## Installation

```bash
go install github.com/mktcz/wisp@latest
```

Or build from source:

```bash
git clone https://github.com/mktcz/wisp.git
cd wisp
go build -o wisp .
sudo cp wisp /usr/local/bin/wisp
```

## Quick Start

1. **Initialize a sample configuration:**

   ```bash
   wisp init
   ```

2. **Customize `wisp.toml` for your project:**

   ```toml
   # API Server
   [api]
     run_cmd = "/tmp/api-server"
     build_cmd = "go build -o /tmp/api-server ./cmd/api"
     watch_dir = "./cmd/api"
     env = { PORT = "8080" }

   # Background Worker
   [worker]
     run_cmd = "/tmp/worker"
     build_cmd = "go build -o /tmp/worker ./cmd/worker"
     watch_dir = "./cmd/worker"
     env = { QUEUE_PROVIDER = "redis" }
   ```

3. **Run your applications:**

   ```bash
   # Run all applications
   wisp

   # Run a specific application
   wisp run api
   ```

## Commands

| Command          | Description                               |
| ---------------- | ----------------------------------------- |
| `wisp`           | Run all applications defined in wisp.toml |
| `wisp init`      | Create a sample wisp.toml configuration   |
| `wisp run <app>` | Run a specific application                |
| `wisp --help`    | Show help message                         |
| `wisp --version` | Show version information                  |

## Configuration

### Basic Options

| Field       | Description                          | Default  |
| ----------- | ------------------------------------ | -------- |
| `run_cmd`   | Command to execute after build       | -        |
| `bin`       | Binary path (alternative to run_cmd) | -        |
| `args`      | Arguments for the binary             | `[]`     |
| `build_cmd` | Command to build the application     | -        |
| `cmd`       | Alias for build_cmd                  | -        |
| `watch_dir` | Directory to watch for changes       | `"."`    |
| `tmp_dir`   | Temporary directory path             | `"/tmp"` |
| `env`       | Environment variables                | `{}`     |

### Timing Configuration

| Field         | Description                          | Default |
| ------------- | ------------------------------------ | ------- |
| `delay`       | Delay before starting (ms)           | `1000`  |
| `kill_delay`  | Delay after stopping (e.g., "500ms") | -       |
| `rerun`       | Rerun even if build fails            | `false` |
| `rerun_delay` | Delay before rerun (ms)              | `500`   |

### Watch Exclusions

| Field            | Description               | Default |
| ---------------- | ------------------------- | ------- |
| `exclude_dir`    | Directories to exclude    | `[]`    |
| `exclude_file`   | File patterns to exclude  | `[]`    |
| `exclude_regex`  | Regex patterns to exclude | `[]`    |
| `follow_symlink` | Follow symbolic links     | `false` |

### Command Hooks

| Field      | Description                  | Default |
| ---------- | ---------------------------- | ------- |
| `pre_cmd`  | Commands to run before build | `[]`    |
| `post_cmd` | Commands to run after build  | `[]`    |

### Process Control

| Field            | Description                    | Default |
| ---------------- | ------------------------------ | ------- |
| `send_interrupt` | Send SIGINT instead of SIGTERM | `false` |
| `stop_on_error`  | Stop watching on error         | `false` |
| `log_silent`     | Suppress application output    | `false` |
| `clean_on_exit`  | Clean tmp files on exit        | `false` |

### Example Configuration

```toml
# wisp.toml

[api]
  run_cmd = "/tmp/api-server"
  build_cmd = "go build -o /tmp/api-server ./cmd/api"
  watch_dir = "./cmd/api"
  env = { PORT = "8080", GIN_MODE = "debug" }

[worker]
  run_cmd = "/tmp/worker"
  build_cmd = "go build -o /tmp/worker ./cmd/worker"
  watch_dir = "./cmd/worker"
  env = { QUEUE_PROVIDER = "redis" }

[frontend]
  # Wisp isn't just for Go!
  run_cmd = "npm run dev"
  watch_dir = "./frontend"
```

## 🤝 Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## 📄 License

MIT License - see LICENSE file for details

## 🙏 Acknowledgments

Wisp was inspired by tools like Air and Nodemon but designed specifically for Go development with a focus on simplicity and concurrent service management.
