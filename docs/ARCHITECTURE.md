# Architecture and ownership boundaries

`gguf-observability` is a read-only evidence plane beside the deployable
platform layers. It consumes public operational interfaces and owns no cluster
resources.

```text
edge-cli (workflow control plane)
  |
  +--> k3s-nvidia-edge (conditional NVIDIA infrastructure)
  |      |
  |      +--> RuntimeClass/nvidia, nvidia.com/gpu, DCGM
  |
  +--> llm-observability-stack (Ollama, GGUF models, WebUI, telemetry)
          |
          +--> kubectl / Helm / Ollama status
                         |
                         v
                  gguf-observability
              read-only contract + evidence
```

## Inputs

- Kubernetes API through `kubectl` for node, pod, and RuntimeClass facts.
- Helm status for the application release state.
- Ollama CLI inside the existing pod for model inventory, residency, and
  effective parameters.
- Host `nvidia-smi` for actual device memory and utilization.
- Operator-selected model alias, VRAM ceiling, and optional parameter contract.

## Outputs

- Human-readable validation results.
- A versioned, sanitized JSON evidence document.
- Markdown generated from that JSON document.
- Optional explicit inference smoke-test evidence without prompt or response
  content.

## Non-goals

- Installing, upgrading, or deleting Kubernetes resources.
- Pulling, creating, stopping, or deleting Ollama models.
- Defining Helm values or Modelfiles.
- Replacing metrics pipelines, Grafana, Prometheus, OpenTelemetry, or DCGM.
- Storing GGUF binaries, credentials, Secrets, kubeconfigs, logs, prompts,
  responses, or raw cluster dumps.

This separation prevents drift: deployment policy remains in
`llm-observability-stack`, infrastructure policy remains in `k3s-nvidia-edge`,
and organization orchestration remains in `edge-cli`.
