# Live validation — July 18, 2026

The native Go observer was validated after a clean Layer 1 and Layer 2 Helm
redeployment. The observer itself remained read-only.

## Environment

- Ubuntu 24.04.3 LTS, kernel 6.17.0-40-generic.
- k3s v1.36.2+k3s1, one Ready control-plane/worker node.
- NVIDIA driver 580.95.05, GeForce 940M, 1,024 MiB VRAM.
- `RuntimeClass/nvidia` present.
- One `nvidia.com/gpu` in node capacity and allocatable resources.
- `k3s-nvidia-edge` Helm release deployed in `gpu-operator`.
- `llm-observability-stack` chart 0.3.0, Helm revision 1, deployed in
  `llm-observability`.
- Ollama, Open WebUI, Redis, and OpenTelemetry Collector Ready with zero
  application-container restarts after the Qwen rollout.

## Qwen state

- Runtime alias: `qwen-1-8b-chat-q4-k-m-local:latest`.
- Size: approximately 1.2 GB.
- Processor split: 27% CPU / 73% GPU.
- CUDA offload: 23/25 layers.
- Parameters: `num_gpu=23`, `num_ctx=256`, `num_batch=1`.
- Residency: `Forever`.
- Device memory: 824 MiB used, 152 MiB free.

The generated snapshot under `evidence/` is the machine-readable record. It is
sanitized and intentionally narrower than a full Kubernetes support bundle.
All 13 runtime-contract checks passed, and the explicit inference smoke test
returned `qwen observation ok`.
