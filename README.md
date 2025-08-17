# GPT-OSS Adapter

This adapter acts as a middleware layer for GPT-OSS models to manage
chain-of-thought (CoT) reasoning between client applications and
inference servers.

## Overview

GPT-OSS models require chain-of-thought (CoT) reasoning from prior tool calls
to maximize their effectiveness. However, not many clients send this back in
their requests. This project solves that by:

- **Field Translation**: Automatically translates reasoning fields between
  different formats to ensure compatibility with various client tools

- **Caching**: Stores reasoning content from tool call responses and
  automatically injects it into subsequent requests when needed

- **Reverse Proxying**: Stands between clients and llama-server/llama-swap to
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

The adapter supports the following OpenAI-compatible endpoints:

- `/v1/chat/completions`
- `/chat/completions`

All other endpoints are proxied directly to the target server without modification.

## License

MIT
