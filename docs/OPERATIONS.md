# Operations

## Normal sequence

1. Deploy or repair the substrate with `k3s-nvidia-edge`.
2. Deploy one model profile with `llm-observability-stack`.
3. Confirm the selected model is resident with `ollama ps`.
4. Run this repository's read-only contract validation with the matching model
   alias and expected parameters.
5. Capture sanitized JSON only when durable point-in-time evidence is needed.
6. Run an inference smoke test only when application-level proof is required.

Validate models sequentially on a 1 GiB GPU. `OLLAMA_MAX_LOADED_MODELS=1`
prevents concurrent runners from competing for device memory, but the operator
should still confirm the previous model is unloaded before changing profiles.

## Failure routing

- Node, GPU resource, or RuntimeClass failure: diagnose `k3s-nvidia-edge`.
- Helm or pod readiness failure: diagnose `llm-observability-stack`.
- Missing or wrong parameters: inspect the selected Modelfile and Helm overlay
  in `llm-observability-stack`.
- VRAM above the ceiling: confirm no unrelated host process or second model is
  using the GPU, then lower `num_gpu` in the deployment owner repository.
- Evidence parsing failure: open an issue with redacted command output; never
  attach kubeconfig, Secrets, prompts, or model responses.

## Safety

`validate`, `snapshot`, and `report` do not mutate the cluster. `smoke` submits
one inference request to the already-running model. None of the commands call
`ollama pull`, `ollama create`, `ollama stop`, `ollama rm`, Helm mutation, or
`kubectl apply`.
