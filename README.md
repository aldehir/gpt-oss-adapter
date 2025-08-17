# GPT-OSS Adapter

A middleware proxy that manages chain-of-thought reasoning for GPT-OSS models.

## Overview

This adapter sits between client applications and inference servers to manage
chain-of-thought reasoning for GPT-OSS models. GPT-OSS models work better when
they receive reasoning context from prior tool calls, but most clients don't
include this context in subsequent requests. It handles this by:

- **Provider Support**: Automatically maps fields based on the target provider
  (LM Studio, llama.cpp)

- **Reasoning Effort**: Extracts `reasoning.effort` from requests and maps it
  to the appropriate provider field

- **Caching**: Stores reasoning content from tool call responses and
  automatically injects it into subsequent requests

- **Reverse Proxying**: Sits between clients and inference servers to
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
- `--provider, -p`: Target provider type (lmstudio, llamacpp)

## Provider Support

The adapter automatically handles field mapping based on the target provider:

### LM Studio (`lmstudio`)
- **Reasoning field**: `reasoning`
- **Reasoning effort**: `reasoning_effort`

### llama.cpp (`llamacpp`)
- **Reasoning field**: `reasoning_content`
- **Reasoning effort**: `chat_template_kwargs.reasoning_effort`

## Reasoning Effort Support

The adapter automatically extracts `reasoning.effort` from client requests and
maps it to the appropriate provider field:

- **Input**: `{"reasoning": {"effort": "high"}}`
- **LM Studio**: Maps to `reasoning_effort`
- **llama.cpp**: Maps to `chat_template_kwargs.reasoning_effort`

### Examples

```bash
# Proxy requests to LM Studio
gpt-oss-adapter \
  --target http://localhost:1234 \
  --listen :8005 \
  --provider lmstudio \
  --verbose

# Proxy requests to llama.cpp server
gpt-oss-adapter \
  --target http://localhost:8000 \
  --listen :8005 \
  --provider llama-cpp \
  --verbose
```

### Supported Endpoints

The adapter handles these OpenAI-compatible endpoints:

- `/v1/chat/completions`
- `/chat/completions`

Other endpoints pass through unchanged.

## License

MIT
