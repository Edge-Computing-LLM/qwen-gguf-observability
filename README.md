# qwen-gguf-observability

Read-only runtime validation and sanitized evidence capture for a Qwen GGUF
model served by Ollama on local Ubuntu + k3s + NVIDIA GPU systems.

This repository documents the currently verified Qwen 1.8B Chat Q4_K_M runtime
and provides a dependency-free Go observer. It answers operational
questions such as:

- Is the k3s node Ready and advertising `nvidia.com/gpu`?
- Is `RuntimeClass/nvidia` available?
- Is the application Helm release deployed and are its pods Ready?
- Is the expected Qwen alias registered, GPU-backed, and kept resident?
- Are the low-memory parameters still `num_gpu=23`, `num_ctx=256`, and
  `num_batch=1`?
- Is total observed VRAM usage still at or below the 850 MiB safety ceiling?

## Repository role

This is an evidence companion, not another deployment layer:

| Repository | Ownership |
|---|---|
| [`edge-cli`](https://github.com/Edge-Computing-LLM/edge-cli) | Organization control plane and ordered install/validate workflows |
| [`k3s-nvidia-edge`](https://github.com/Edge-Computing-LLM/k3s-nvidia-edge) | Ubuntu, k3s, NVIDIA runtime, GPU Operator, device plugin, and DCGM substrate |
| [`llm-observability-stack`](https://github.com/Edge-Computing-LLM/llm-observability-stack) | Ollama/GGUF, Open WebUI, OpenTelemetry, Helm profiles, dashboards, and deployment configuration |
| `qwen-gguf-observability` | Read-only Qwen runtime contract checks and sanitized point-in-time evidence |
| Grafana dashboards in `llm-observability-stack` | Presentation of GPU, Kubernetes, Ollama, and LLM observability data |

It deliberately does not install k3s, NVIDIA software, Helm releases, Ollama,
models, Open WebUI, or telemetry backends. It does not contain model weights or
duplicate Modelfiles and Helm values. Those remain in their owning repositories.

## Verified local runtime

Observed on July 18, 2026:

| Component | Value |
|---|---|
| Host | Ubuntu 24.04.3 LTS, ThinkPad T450s |
| Kubernetes | k3s v1.36.2+k3s1, single control-plane/worker node |
| GPU | NVIDIA GeForce 940M, 1,024 MiB VRAM |
| NVIDIA resource | `nvidia.com/gpu: 1` capacity and allocatable |
| Model | `qwen-1-8b-chat-q4-k-m-local:latest` |
| Ollama split | 27% CPU / 73% GPU, 23/25 layers offloaded |
| Context and batch | 256 tokens, batch size 1 |
| GPU memory | 824 MiB used, 152 MiB free |
| Residency | `Forever` |
| Application release | `llm-observability-stack`, Helm revision 6, deployed |

The upstream Ollama tag is
[`qwen:1.8b-chat-q4_K_M`](https://ollama.com/library/qwen:1.8b-chat-q4_K_M).
The model license is separate from this repository's MIT license; consult the
upstream Tongyi Qianwen Research License before using the model.

## Requirements

- Go 1.22 or newer for source builds, or the prebuilt `qwen-observe` binary
- `kubectl` configured for the target cluster
- `helm`
- host `nvidia-smi`
- a running Ollama pod in the target namespace

The observer uses only the Go standard library. It compiles to one static binary;
no Python runtime, virtual environment, or third-party package is required.
Deployment orchestration remains in Go and this repository does not grow Bash
lifecycle scripts. See the organization
[language boundaries](https://github.com/Edge-Computing-LLM/edge-cli/blob/main/docs/LANGUAGE-BOUNDARIES.md).

## Validate the live runtime

```bash
make build
./bin/qwen-observe validate
```

The command is read-only. It exits non-zero if any runtime contract fails.

## Capture sanitized evidence

```bash
./bin/qwen-observe snapshot \
  --output evidence/live-snapshot.json

./bin/qwen-observe report \
  --input evidence/live-snapshot.json \
  --output evidence/live-report.md
```

Snapshots contain selected versions, readiness, GPU memory, model residency,
and parameter facts. They intentionally exclude Kubernetes Secrets, kubeconfig
content, environment variables, logs, prompts, model weights, and host IPs.

## Run an explicit inference smoke test

```bash
./bin/qwen-observe smoke \
  --prompt "Reply with exactly: qwen observation ok" \
  --expect "qwen observation ok" \
  --output evidence/smoke.json
```

Unlike `validate` and `snapshot`, `smoke` performs inference. It does not stop or
unload the model; the deployed `OLLAMA_KEEP_ALIVE=-1` policy keeps it resident.

## Test the project

```bash
make check
```

See [architecture](docs/ARCHITECTURE.md), [runtime contract](docs/RUNTIME-CONTRACT.md),
[operations](docs/OPERATIONS.md), and the
[July 18 live validation](docs/LIVE-VALIDATION-2026-07-18.md).
