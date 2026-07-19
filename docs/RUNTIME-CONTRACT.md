# GGUF runtime contract

The observer evaluates a model-selectable contract for a local low-VRAM NVIDIA
profile. A failed check reports runtime drift and never modifies the cluster.

| Check | Expected state | Owning repository |
|---|---|---|
| Kubernetes node | Ready | `k3s-nvidia-edge` / k3s |
| GPU capacity | At least one allocatable `nvidia.com/gpu` | `k3s-nvidia-edge` |
| RuntimeClass | `nvidia` handler exists | `k3s-nvidia-edge` |
| Helm release | `llm-observability-stack` is deployed | `llm-observability-stack` |
| Application pods | All observed containers Ready | `llm-observability-stack` |
| Model registration | Selected model alias exists | `llm-observability-stack` |
| Model residency | Selected model appears in `ollama ps` | Ollama runtime |
| GPU participation | Processor string contains `GPU` | Ollama runtime |
| Keep-alive | `Until=Forever` | `llm-observability-stack` |
| GPU memory | Total device use is at most the selected MiB ceiling | Host NVIDIA runtime |
| Effective parameters | Selected values match when checks are enabled | Model Modelfile |

The default ceiling is 900 MiB. It is an observed operational guardrail, not a
hardware partition: this GeForce GPU does not provide MIG, and Kubernetes's
`nvidia.com/gpu` resource allocates the whole device. `num_gpu` controls the
number of model layers offloaded to CUDA; remaining layers execute from system
RAM. Total `nvidia-smi` usage also includes small host display allocations.

Example:

```bash
GGUF_MODEL=qwen-1-8b-chat-q4-k-m-local \
GGUF_VRAM_CEILING_MIB=900 \
GGUF_EXPECTED_NUM_GPU=23 \
GGUF_EXPECTED_NUM_CTX=256 \
GGUF_EXPECTED_NUM_BATCH=1 \
./bin/gguf-observe validate
```

Set an expected parameter to `-1` to omit that one parameter assertion while
retaining registration, residency, GPU participation, and VRAM checks.
