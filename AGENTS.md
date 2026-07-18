# Repository instructions

This repository is a read-only Go evidence plane. It may inspect Kubernetes,
Helm, Ollama, and NVIDIA status but must not create, patch, restart, remove, or
otherwise mutate cluster resources or models. Preserve evidence schema `1.0`
unless a deliberate versioned migration is documented.

Before completing a change run `gofmt`, `go test ./...`, `go vet ./...`, and
build `cmd/qwen-observe`. Never include Secrets, kubeconfig data, host IPs, pod
logs, prompts, responses, environment dumps, or model weights in snapshots.
