# Zed Editor Compatibility Diagnosis

## Summary

**copilot-proxy is NOT compatible with Zed Editor's Ollama handler** due to fundamental differences in the API response format for the `/api/chat` endpoint.

---

## Root Cause Analysis

### The Problem

Zed Editor expects a **native Ollama API response format**, but copilot-proxy proxies to Z.AI (an OpenAI-compatible API), which returns **OpenAI-format responses**.

### How Zed's Ollama Handler Works

Looking at [ollama.rs](https://github.com/zed-industries/zed/blob/main/crates/ollama/src/ollama.rs), Zed expects:

1. **`/api/tags`** (GET) - List available models
2. **`/api/show`** (POST) - Get model capabilities and context length
3. **`/api/chat`** (POST) - Stream chat completions

---

## Detailed Endpoint Comparison

### 1. `/api/tags` - ✅ Compatible

| Feature         | Ollama API                                                  | copilot-proxy   |
| --------------- | ----------------------------------------------------------- | --------------- |
| Response format | `{"models": [...]}`                                         | ✅ Same         |
| Model fields    | `name`, `model`, `modified_at`, `size`, `digest`, `details` | ✅ Provides all |

### 2. `/api/show` - ⚠️ Partially Compatible

| Feature                               | Ollama API (Zed expects)            | copilot-proxy            |
| ------------------------------------- | ----------------------------------- | ------------------------ |
| `capabilities`                        | `["completion", "tools", "vision"]` | ✅ `["tools", "vision"]` |
| `model_info["general.architecture"]`  | Required (e.g., `"llama"`)          | ✅ `"glm"`               |
| `model_info["{arch}.context_length"]` | Dynamic key based on architecture   | ✅ `glm.context_length`  |

> **Note:** Zed's parser dynamically reads `model_info["{architecture}.context_length"]`, so for `architecture = "glm"`, it looks for `glm.context_length`.

### 3. `/api/chat` - ❌ NOT Compatible (Critical Issue)

This is the **critical incompatibility**.

#### Ollama Response Format (What Zed Expects)

Streaming response - each line is a JSON object:

```json
{
    "model": "llama3.2",
    "created_at": "2023-08-04T08:52:19.385406455-07:00",
    "message": {
        "role": "assistant",
        "content": "The"
    },
    "done": false
}
```

Final message:

```json
{
    "model": "llama3.2",
    "created_at": "2023-08-04T19:22:45.499127Z",
    "message": {
        "role": "assistant",
        "content": ""
    },
    "done": true,
    "total_duration": 4883583458,
    "load_duration": 1334875,
    "prompt_eval_count": 26,
    "prompt_eval_duration": 342546000,
    "eval_count": 282,
    "eval_duration": 4535599000
}
```

#### OpenAI Response Format (What copilot-proxy Returns)

Since copilot-proxy proxies to `/v1/chat/completions` (OpenAI-compatible endpoint), it returns:

```
data: {"id":"...","object":"chat.completion.chunk","choices":[{"delta":{"content":"The"}}]}
data: {"id":"...","object":"chat.completion.chunk","choices":[{"delta":{"content":" sky"}}]}
data: [DONE]
```

**Key differences:**

| Feature            | Ollama Format                    | OpenAI Format              |
| ------------------ | -------------------------------- | -------------------------- |
| Streaming protocol | NDJSON (one JSON per line)       | SSE (`data: {...}`)        |
| Message structure  | `message.content`                | `choices[0].delta.content` |
| Done indicator     | `"done": true` JSON field        | `data: [DONE]` line        |
| Metadata           | Duration stats in final response | None                       |

---

## Zed's Response Parsing

From `ollama.rs`:

```rust
pub async fn stream_chat_completion(...) {
    Ok(reader
        .lines()
        .map(|line| match line {
            Ok(line) => serde_json::from_str(&line).context("Unable to parse chat response"),
            Err(e) => Err(e.into()),
        })
        .boxed())
}
```

Zed reads **lines** and parses each line as JSON directly. OpenAI SSE format (`data: {...}`) would fail this parsing.

---

## Additional Request Format Issues

| Parameter          | Ollama              | OpenAI                   |
| ------------------ | ------------------- | ------------------------ |
| Context management | `keep_alive`        | N/A                      |
| Model options      | `options: {}`       | Various top-level params |
| Thinking mode      | `think: true/false` | N/A                      |

---

## Recommended Solution

To make copilot-proxy compatible with Zed Editor, implement a **response format translator**:

1. **For `/api/chat`**:
    - Accept Ollama-format requests
    - Translate to OpenAI format before sending upstream
    - Parse SSE responses from Z.AI
    - Re-encode as NDJSON in Ollama format

### Architecture

```
Zed Editor ─┬─► /api/tags       ─► Static catalog (✅ works)
            ├─► /api/show       ─► Static metadata (✅ works)
            └─► /api/chat       ─► Need format translation layer
                    │
                    ▼
            ┌─────────────────────────┐
            │  Response Translator    │
            │  Ollama → OpenAI req    │
            │  OpenAI → Ollama resp   │
            └─────────────────────────┘
                    │
                    ▼
              Z.AI API (OpenAI format)
```

---

## Effort Estimate

| Task                                     | Complexity    |
| ---------------------------------------- | ------------- |
| Request translation (Ollama → OpenAI)    | Moderate      |
| SSE to NDJSON streaming                  | Moderate-High |
| Response field mapping                   | Moderate      |
| Metadata generation (`done`, timestamps) | Easy          |

---

## Conclusion

The copilot-proxy cannot work with Zed Editor's Ollama handler because:

1. **Response format mismatch**: Zed expects NDJSON, copilot-proxy returns SSE
2. **Message structure mismatch**: Zed parses `message.content`, OpenAI uses `choices[].delta.content`
3. **Done signaling mismatch**: Zed expects `"done": true` in JSON, OpenAI uses `data: [DONE]`
