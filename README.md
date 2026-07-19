# gguf-observability

Read-only runtime validation and sanitized evidence capture for GGUF models
served by Ollama on local Ubuntu + k3s + NVIDIA GPU systems.

The dependency-free Go observer accepts a model alias and runtime contract at
execution time. It is designed for Qwen, Gemma, Llama, and future GGUF-backed
Ollama models without embedding vendor-specific logic in the evidence schema.

It answers operational questions such as:

- Is the k3s node Ready and advertising `nvidia.com/gpu`?
- Is `RuntimeClass/nvidia` available?
- Is the application Helm release deployed and are its pods Ready?
- Is the selected model registered, GPU-backed, and kept resident?
- Do its effective `num_gpu`, `num_ctx`, and `num_batch` parameters match the
  selected low-memory contract?
- Is total observed GPU memory at or below the 900 MiB operational ceiling?

## Repository role

This is an evidence companion, not another deployment layer:

| Repository | Ownership |
|---|---|
| [`edge-cli`](https://github.com/Edge-Computing-LLM/edge-cli) | Organization control plane and ordered install/validate workflows |
| [`k3s-nvidia-edge`](https://github.com/Edge-Computing-LLM/k3s-nvidia-edge) | Ubuntu, k3s, NVIDIA runtime, GPU Operator, device plugin, and DCGM substrate |
| [`llm-observability-stack`](https://github.com/Edge-Computing-LLM/llm-observability-stack) | Ollama/GGUF, model Modelfiles, Open WebUI, OpenTelemetry, Helm profiles, dashboards, and deployment configuration |
| `gguf-observability` | Read-only, model-selectable runtime contracts and sanitized point-in-time evidence |

It does not install k3s, NVIDIA software, Helm releases, Ollama, models, Open
WebUI, or telemetry backends. It does not contain model weights or duplicate
Modelfiles and Helm values.

## Initial model catalog

| Model alias | Source | Contract profile |
|---|---|---|
| `qwen-1-8b-chat-q4-k-m-local` | Local Qwen 1.8B Chat Q4_K_M GGUF | `num_gpu=23`, `num_ctx=256`, `num_batch=1` |
| `gemma3-1b-it-gguf-local` | Local Gemma 3 1B IT Q4_K_M GGUF | `num_gpu=23`, `num_ctx=256`, `num_batch=1` |
| `llama3-2-1b-local` | Official Ollama `llama3.2:1b` GGUF-backed model | Tuned profile documented in `docs/MODEL-PROFILES.md` |

Model licenses remain separate from this repository's MIT license. Consult the
respective upstream model terms before downloading or using a model.

## Requirements

- Go 1.22 or newer for source builds, or the prebuilt `gguf-observe` binary
- `kubectl` configured for the target cluster
- `helm`
- host `nvidia-smi`
- a running Ollama pod in the target namespace

The observer uses only the Go standard library. It compiles to one static
binary; no Python runtime or third-party package is required.

## Validate a live model

```bash
make build
./bin/gguf-observe \
  --model qwen-1-8b-chat-q4-k-m-local \
  --vram-ceiling-mib 900 \
  --expected-num-gpu 23 \
  --expected-num-ctx 256 \
  --expected-num-batch 1 \
  validate
```

Use `-1` for any expected parameter that should not be checked. The command is
read-only and exits non-zero when a runtime contract fails.

## Capture sanitized evidence

```bash
./bin/gguf-observe --model gemma3-1b-it-gguf-local snapshot \
  --output evidence/live-snapshot.json

./bin/gguf-observe report \
  --input evidence/live-snapshot.json \
  --output evidence/live-report.md
```

Snapshots contain selected versions, readiness, GPU memory, model residency,
and parameter facts. They exclude Kubernetes Secrets, kubeconfig content,
environment variables, logs, prompts, responses, model weights, and host IPs.

## Run an explicit inference smoke test

```bash
./bin/gguf-observe --model llama3-2-1b-local smoke \
  --prompt "Reply with exactly: observation ok" \
  --expect "observation ok" \
  --output evidence/smoke.json
```

The response is printed for the operator but is never serialized in the smoke
evidence. The smoke command does not stop or unload the model.

## Extend the catalog

Adding a model does not require Go source changes. Add its deployment Modelfile
and Helm overlay to `llm-observability-stack`, record its contract in
[`docs/MODEL-PROFILES.md`](docs/MODEL-PROFILES.md), then pass the model alias
and expected parameters to `gguf-observe`.

Run all repository checks with:

```bash
make check
```

See [architecture](docs/ARCHITECTURE.md), [runtime contract](docs/RUNTIME-CONTRACT.md),
[operations](docs/OPERATIONS.md), and the retained historical Qwen validation
under `docs/LIVE-VALIDATION-2026-07-18.md`.
