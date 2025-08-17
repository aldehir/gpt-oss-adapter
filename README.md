# GPT-OSS Adapter

A middleware proxy that manages chain-of-thought reasoning for GPT-OSS models.

## Overview

This adapter sits between client applications and inference servers to manage
chain-of-thought reasoning for GPT-OSS models. GPT-OSS models work better when
they receive reasoning context from prior tool calls, but most clients don't
include this context in subsequent requests. It handles this by:

- **Field Translation**: Translates reasoning fields between different formats
  for compatibility with various client tools

- **Caching**: Stores reasoning content from tool call responses and
  automatically injects it into subsequent requests

- **Reverse Proxying**: Sits between clients and llama-server/llama-swap to
  manage reasoning content

## Installation

Build from source:

```bash
go build -o gpt-oss-adapter
```

## Usage

```bash
gpt-oss-adapter --target http://localhost:8080 --listen :8005
```

### Command Line Options

- `--target, -t`: Target server URL (required)
- `--listen, -l`: Server listen address (default: `:8005`)
- `--verbose, -v`: Enable debug logging
- `--reasoning-from`: Source field name for reasoning content (default: `reasoning_content`)
- `--reasoning-to`: Target field name for reasoning content (default: `reasoning`)

### Example

```bash
# Proxy requests to llama-server with custom reasoning field mapping
gpt-oss-adapter \
  --target http://localhost:8000 \
  --listen :8005 \
  --reasoning-from reasoning_content \
  --reasoning-to reasoning \
  --verbose
```

### Supported Endpoints

The adapter handles these OpenAI-compatible endpoints:

- `/v1/chat/completions`
- `/chat/completions`

Other endpoints pass through unchanged.

## License

MIT
