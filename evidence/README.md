# Evidence directory

The dated JSON and Markdown files committed here are sanitized observations of
the verified local runtime. Generated convenience names such as
`live-snapshot.json`, `live-report.md`, and `smoke.json` are ignored so routine
local checks do not dirty the repository.

Never commit model files, Secrets, kubeconfigs, access tokens, full pod logs,
raw cluster dumps, prompts, or model responses. Smoke JSON records only timing,
model identity, and pass/fail status; the response is console-only.
