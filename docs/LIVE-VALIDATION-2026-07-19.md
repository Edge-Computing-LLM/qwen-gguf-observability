# Three-model live validation — July 19, 2026

Gemma 3 1B IT Q4_K_M, Qwen 1.8B Chat Q4_K_M, and Llama 3.2 1B were deployed
sequentially through `llm-observability-stack` on the local Ubuntu + k3s +
NVIDIA reference system.

## Clean deployment sequence

The prior application and NVIDIA Helm releases were removed. Core k3s services
and host GGUF files were preserved. The NVIDIA toolkit removal restarted k3s,
after which the API returned Ready. `k3s-nvidia-edge` was then reinstalled from
its local chart; its operator, Node Feature Discovery, toolkit, device plugin,
DCGM exporter, and validator workloads all became healthy, and the CUDA
validator completed.

The prior application PVC was removed by the Helm uninstall despite the intent
to retain runtime data. No host GGUF file was deleted. A new 5 GiB Ollama claim
was created, local Gemma and Qwen GGUF models were re-imported, and the official
Ollama `llama3.2:1b` model was downloaded again.

## Results

| Model | Effective parameters | Processor | VRAM | Smoke duration | Contract |
|---|---|---:|---:|---:|---:|
| Gemma 3 1B IT Q4_K_M | `num_gpu=23`, `num_ctx=256`, `num_batch=1` | 62% CPU / 38% GPU | 450 MiB | 1.297 s | 13/13 pass |
| Qwen 1.8B Chat Q4_K_M | `num_gpu=23`, `num_ctx=256`, `num_batch=1` | 27% CPU / 73% GPU | 824 MiB | 1.932 s | 13/13 pass |
| Llama 3.2 1B | `num_gpu=8`, `num_ctx=256`, `num_batch=1` | 52% CPU / 48% GPU | 541 MiB | 1.591 s | 13/13 pass |

All total-device readings remained below the 900 MiB operational ceiling.
`ollama ps` showed exactly one resident model after every profile change, with
`Until=Forever`. The final deployed model is `llama3-2-1b-local`.

## Evidence boundary

Sanitized JSON snapshots, generated Markdown reports, and smoke metadata are in
`evidence/validation-2026-07-19/`. Smoke files include model identity, time,
duration, and pass state only. Prompts and responses were not serialized.

These observations are specific to this machine and point in time. They do not
claim equivalent speed, memory use, or stability on other GPUs, Ollama
versions, drivers, kernels, or workloads.
