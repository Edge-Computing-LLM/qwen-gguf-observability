# Operations

## Normal sequence

1. Deploy or repair the platform with `edge-cli`.
2. Run `edge validate infra` and `edge validate observability`.
3. Run this repository's read-only contract validation.
4. Capture sanitized JSON only when a durable point-in-time record is needed.
5. Run an inference smoke test only when application-level proof is required.

## Failure routing

- Node, GPU resource, or RuntimeClass failure: diagnose `k3s-nvidia-edge`.
- Helm or pod readiness failure: diagnose `llm-observability-stack`.
- Missing or wrong model parameters: inspect the Qwen Modelfile and GeForce
  values in `llm-observability-stack`.
- VRAM above the ceiling: first confirm no unrelated host process or second
  model is using the GPU. Do not automatically unload the production model.
- Evidence-tool parsing failure: open an issue in this repository with redacted
  command output; never attach kubeconfig or Secrets.

## Safety

`validate`, `snapshot`, and `report` do not mutate the cluster. `smoke` submits
one inference request to the already-running model. None of the commands call
`ollama stop`, `ollama rm`, Helm install/upgrade/uninstall, or `kubectl apply`.
