# Live GGUF model observation

Observed at `2026-07-19T07:07:43Z` using evidence schema `1.0`.

## Runtime

| Field | Observed value |
|---|---|
| Host | Ubuntu 24.04.3 LTS |
| Kubernetes | v1.36.2+k3s1 |
| Node | node-1 (Ready: true) |
| GPU | NVIDIA GeForce 940M (1024 MiB) |
| GPU memory | 541 MiB used / 435 MiB free |
| Model | `llama3-2-1b-local` |
| Processor split | 52%/48% CPU/GPU |
| Context | 256 |
| Residency | Forever |
| Helm release | llm-observability-stack (deployed) |

## Checks

| Check | Result | Contract |
|---|---|---|
| kubernetes-node-ready | PASS | all observed nodes are Ready |
| nvidia-gpu-allocatable | PASS | at least one nvidia.com/gpu is allocatable |
| nvidia-runtimeclass | PASS | RuntimeClass/nvidia exists |
| helm-release-deployed | PASS | LLM stack Helm release is deployed |
| workloads-ready | PASS | all observed application containers are Ready |
| model-registered | PASS | expected Ollama model alias is registered |
| model-resident | PASS | expected Ollama model alias is loaded |
| model-gpu-active | PASS | Ollama reports GPU participation |
| model-keep-alive | PASS | Ollama reports Until=Forever |
| vram-ceiling | PASS | total observed GPU memory stays at or below 900 MiB |
| num-gpu-layers | PASS | num_gpu is 8 |
| context-window | PASS | num_ctx is 256 |
| batch-size | PASS | num_batch is 1 |

This report contains selected operational facts only. It excludes Secrets,
kubeconfig data, environment variables, pod logs, prompts, responses, and model weights.
