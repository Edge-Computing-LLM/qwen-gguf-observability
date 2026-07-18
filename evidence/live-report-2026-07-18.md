# Live Qwen GGUF observation

Observed at `2026-07-18T07:28:33Z` using evidence schema `1.0`.

## Runtime

| Field | Observed value |
|---|---|
| Host | Ubuntu 24.04.3 LTS |
| Kubernetes | v1.36.2+k3s1 |
| Node | node-1 (Ready: true) |
| GPU | NVIDIA GeForce 940M (1024 MiB) |
| GPU memory | 824 MiB used / 152 MiB free |
| Model | `qwen-1-8b-chat-q4-k-m-local` |
| Processor split | 27%/73% CPU/GPU |
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
| qwen-registered | PASS | expected local Qwen alias is registered |
| qwen-resident | PASS | expected local Qwen alias is loaded |
| qwen-gpu-active | PASS | Ollama reports GPU participation |
| qwen-keep-alive | PASS | Ollama reports Until=Forever |
| vram-ceiling | PASS | GPU memory stays at or below 850 MiB |
| num-gpu-layers | PASS | num_gpu is 23 |
| context-window | PASS | num_ctx is 256 |
| batch-size | PASS | num_batch is 1 |

This report contains selected operational facts only. It excludes Secrets,
kubeconfig data, environment variables, pod logs, and model weights.
