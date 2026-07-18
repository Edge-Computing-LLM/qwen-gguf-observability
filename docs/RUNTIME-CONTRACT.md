# Qwen runtime contract

The observer evaluates a narrow contract for the validated GeForce 940M
profile. A failed check indicates runtime drift; it does not automatically
modify the cluster.

| Check | Expected state | Owning repository |
|---|---|---|
| Kubernetes node | Ready | `k3s-nvidia-edge` / k3s |
| GPU capacity | At least one allocatable `nvidia.com/gpu` | `k3s-nvidia-edge` |
| RuntimeClass | `nvidia` handler exists | `k3s-nvidia-edge` |
| Helm release | `llm-observability-stack` is deployed | `llm-observability-stack` |
| Application pods | All containers Ready | `llm-observability-stack` |
| Model registration | Qwen local alias exists | `llm-observability-stack` |
| Model residency | Qwen appears in `ollama ps` | `llm-observability-stack` |
| GPU participation | Processor string contains `GPU` | Ollama runtime |
| Keep-alive | `Until=Forever` | `llm-observability-stack` |
| GPU memory | At most 850 MiB total observed use | GeForce profile |
| GPU layers | `num_gpu=23` | Qwen Modelfile |
| Context | `num_ctx=256` | Qwen Modelfile |
| Batch | `num_batch=1` | Qwen Modelfile |

The 850 value is MiB, not GB: the physical GeForce 940M has 1,024 MiB. Total
device usage includes the Ollama runner and small host display allocations.

Override names and the ceiling without editing source:

```bash
QWEN_NAMESPACE=llm-observability \
QWEN_RELEASE=llm-observability-stack \
QWEN_MODEL=qwen-1-8b-chat-q4-k-m-local \
QWEN_VRAM_CEILING_MIB=850 \
./bin/qwen-observe validate
```
