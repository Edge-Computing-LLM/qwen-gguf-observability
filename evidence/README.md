# Evidence directory

The dated JSON and Markdown files committed here are sanitized observations of
the verified local runtime. Generated convenience names such as
`live-snapshot.json`, `live-report.md`, and `smoke.json` are ignored so routine
local checks do not dirty the repository.

The `validation-2026-07-19/` directory contains model-qualified snapshots,
reports, and smoke metadata for Gemma, Qwen, and Llama. Each model passed 13
read-only runtime checks under the 900 MiB observed VRAM ceiling.

Never commit model files, Secrets, kubeconfigs, access tokens, full pod logs,
raw cluster dumps, prompts, or model responses. Smoke JSON records only timing,
model identity, and pass/fail status; the response is console-only.
