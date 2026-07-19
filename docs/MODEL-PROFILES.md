# Model profiles

This catalog records runtime contracts that can be supplied to `gguf-observe`.
Deployment files and model lifecycle remain in `llm-observability-stack`.

| Profile | Ollama alias | Expected parameters | Observed CPU/GPU split | Observed VRAM | Ceiling |
|---|---|---|---:|---:|---:|
| Qwen 1.8B Chat Q4_K_M | `qwen-1-8b-chat-q4-k-m-local` | `num_gpu=23`, `num_ctx=256`, `num_batch=1` | 27% / 73% | 824 MiB | 900 MiB |
| Gemma 3 1B IT Q4_K_M | `gemma3-1b-it-gguf-local` | `num_gpu=23`, `num_ctx=256`, `num_batch=1` | 62% / 38% | 450 MiB | 900 MiB |
| Llama 3.2 1B | `llama3-2-1b-local` | `num_gpu=8`, `num_ctx=256`, `num_batch=1` | 52% / 48% | 541 MiB | 900 MiB |

These are point-in-time observations from July 19, 2026 on the local GeForce
940M system. They are not portable performance guarantees. See
[`LIVE-VALIDATION-2026-07-19.md`](LIVE-VALIDATION-2026-07-19.md).

## Add another model

1. Verify the upstream model name, license, provenance, and checksum where a
   local GGUF is used.
2. Add a Modelfile and a model-selection Helm overlay to
   `llm-observability-stack`.
3. Start with a small `num_gpu`, context, and batch on low-VRAM hardware.
4. Deploy only one loaded model, measure total device use with `nvidia-smi`, and
   increase offloaded layers only while remaining below the chosen ceiling.
5. Run `gguf-observe` with the alias and measured parameters.
6. Record only sanitized facts. Never commit model weights, host paths, pod
   logs, prompts, responses, IP addresses, or credentials.

Historical Qwen evidence predating the repository rename remains clearly
identified as Qwen-specific. New captures should use model-qualified filenames,
for example `snapshot-llama3-2-1b-YYYYMMDD.json`.
