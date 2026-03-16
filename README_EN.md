# KB Runner

KB Script Execution Framework - Execute KB check scripts and generate result matrices.

[中文](./README.md) | English

## Features

### CLI Tool
- 📋 CASE Management - List, filter, search CASEs
- 🎬 Scenario Management - Define and execute scenarios
- ⚡ Script Execution - Support Bash and Python scripts
- 📊 Result Processing - Weighting, normalization, matrix generation
- 📝 Log Standards - Standardized log output
- 📤 Result Export - JSON/YAML format

### Web Interface (P1)
- 🌐 Embedded Web Frontend - Single binary deployment
- 📈 Execution History - Local file storage
- 🔄 Auto Cleanup - Configurable max records
- 🎨 Oatly Style - Hand-drawn illustration style

## Quick Start

### Download

Download the binary for your platform from [Releases](https://github.com/Wlinuxhv/kb-runner/releases).

### CLI Mode

```bash
# Show help
./kb-runner --help

# List all CASEs
./kb-runner list

# Execute script
./kb-runner run -s ./scripts/bash/security_check.sh -l bash

# Execute by CASE
./kb-runner run --case security_check

# Execute by scenario
./kb-runner run --scenario daily_check

# Interactive mode
./kb-runner run --interactive

# Initialize CASE directory
./kb-runner init my_case
```

### Web Interface

```bash
# Start web server
./kb-runner serve

# Specify port
./kb-runner serve --port 8080
```

Visit http://localhost:8080 after starting.

## Project Structure

```
kb-runner/
├── cmd/              # CLI entry point
├── internal/         # Core modules
│   ├── adapter/     # Script adapter
│   ├── api/         # Web API
│   ├── cases/       # CASE management
│   ├── executor/    # Execution engine
│   ├── processor/   # Result processor
│   └── scenario/    # Scenario management
├── pkg/              # Public libraries
├── scripts/          # Script APIs
│   ├── bash/        # Bash API
│   └── python/      # Python API
└── configs/         # Configuration files
```

## Script API

### Bash

```bash
#!/bin/bash
source ./scripts/bash/api.sh

kb_init

step_start "check_xxx"
# Check logic here
result "key" "value"
step_success "Check passed"

kb_save
```

### Python

```python
#!/usr/bin/env python3
from kb_api import kb_init, kb_save, step_start, step_success, result

kb_init()

step_start("check_xxx")
# Check logic here
result("key", "value")
step_success("Check passed")

kb_save()
```

## Configuration

Config file `configs/config.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 8080

execution:
  timeout: 300s
  max_parallel: 10
  work_dir: "./workspace"
  temp_dir: "./temp"

scripts:
  directory: "./scripts"
  allowed_languages:
    - bash
    - python

logging:
  level: "info"
  format: "json"

history:
  enabled: true
  max_records: 4294967296
  auto_cleanup: true
  cleanup_threshold: 0.9
```

## Development

### Build

```bash
# Build for Linux
make build-linux

# Build for Windows
make build-windows

# Build all platforms
make release
```

### Test

```bash
make test
```

## License

MIT License
