# Contributing

Keep this project read-only with respect to Kubernetes and Ollama lifecycle.
Deployment logic belongs in the owning Edge-Computing-LLM repository.

Before submitting changes:

```bash
make check
```

Use Python 3.11 and the standard library unless a dependency has a clear,
documented operational benefit. Do not commit generated caches or sensitive
runtime material.
