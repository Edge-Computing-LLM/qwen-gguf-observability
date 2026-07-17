#!/usr/bin/env python3.11
"""Read-only validation and sanitized evidence capture for Qwen on local k3s."""

from __future__ import annotations

import argparse
import csv
import json
import os
import platform
import re
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Sequence


SCHEMA_VERSION = "1.0"
DEFAULT_NAMESPACE = "llm-observability"
DEFAULT_RELEASE = "llm-observability-stack"
DEFAULT_MODEL = "qwen-1-8b-chat-q4-k-m-local"


class ObservationError(RuntimeError):
    """Raised when a required local observation cannot be collected."""


class Runner:
    def run(self, args: Sequence[str]) -> str:
        try:
            completed = subprocess.run(
                list(args),
                check=True,
                capture_output=True,
                text=True,
            )
        except FileNotFoundError as exc:
            raise ObservationError(f"required command not found: {args[0]}") from exc
        except subprocess.CalledProcessError as exc:
            detail = (exc.stderr or exc.stdout or "no diagnostic output").strip()
            raise ObservationError(f"command failed ({' '.join(args)}): {detail}") from exc
        return completed.stdout.strip()

    def json(self, args: Sequence[str]) -> dict[str, Any]:
        output = self.run(args)
        try:
            value = json.loads(output)
        except json.JSONDecodeError as exc:
            raise ObservationError(f"command did not return JSON: {' '.join(args)}") from exc
        if not isinstance(value, dict):
            raise ObservationError(f"expected JSON object from: {' '.join(args)}")
        return value


def parse_os_release(path: Path = Path("/etc/os-release")) -> dict[str, str]:
    values: dict[str, str] = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        values[key] = value.strip().strip('"')
    return {key.lower(): values[key] for key in ("ID", "VERSION_ID", "PRETTY_NAME") if key in values}


def split_table(output: str) -> list[list[str]]:
    lines = [line.strip() for line in output.splitlines() if line.strip()]
    if len(lines) < 2:
        return []
    return [re.split(r"\s{2,}", line) for line in lines[1:]]


def parse_ollama_list(output: str) -> list[dict[str, str]]:
    models = []
    for fields in split_table(output):
        if len(fields) >= 4:
            models.append({"name": fields[0], "id": fields[1], "size": fields[2], "modified": fields[3]})
    return models


def parse_ollama_ps(output: str) -> list[dict[str, str]]:
    running = []
    for fields in split_table(output):
        if len(fields) >= 6:
            running.append(
                {
                    "name": fields[0],
                    "id": fields[1],
                    "size": fields[2],
                    "processor": fields[3],
                    "context": fields[4],
                    "until": fields[5],
                }
            )
    return running


def parse_parameters(output: str) -> dict[str, Any]:
    parameters: dict[str, Any] = {}
    stops: list[str] = []
    for line in output.splitlines():
        match = re.match(r"^(\S+)\s+(.+?)\s*$", line)
        if not match:
            continue
        key, raw = match.groups()
        value = raw.strip().strip('"')
        if key == "stop":
            stops.append(value)
            continue
        if re.fullmatch(r"-?\d+", value):
            parameters[key] = int(value)
        elif re.fullmatch(r"-?\d+\.\d+", value):
            parameters[key] = float(value)
        else:
            parameters[key] = value
    if stops:
        parameters["stop"] = stops
    return parameters


def ready_condition(item: dict[str, Any]) -> bool:
    conditions = item.get("status", {}).get("conditions", [])
    return any(c.get("type") == "Ready" and c.get("status") == "True" for c in conditions)


def pod_ready(item: dict[str, Any]) -> bool:
    statuses = item.get("status", {}).get("containerStatuses", [])
    return bool(statuses) and all(status.get("ready") is True for status in statuses)


def find_ollama_pod(pods: list[dict[str, Any]]) -> str:
    candidates = []
    for pod in pods:
        metadata = pod.get("metadata", {})
        labels = metadata.get("labels", {})
        if labels.get("app.kubernetes.io/name") == "ollama" or metadata.get("name", "").startswith("ollama-"):
            candidates.append(pod)
    running = [pod for pod in candidates if pod.get("status", {}).get("phase") == "Running"]
    if not running:
        raise ObservationError("no running Ollama pod found")
    return str(running[0]["metadata"]["name"])


def nvidia_snapshot(runner: Runner) -> dict[str, Any]:
    query = "name,driver_version,memory.total,memory.used,memory.free,utilization.gpu,temperature.gpu"
    output = runner.run(["nvidia-smi", f"--query-gpu={query}", "--format=csv,noheader,nounits"])
    row = next(csv.reader([output], skipinitialspace=True))
    if len(row) != 7:
        raise ObservationError("unexpected nvidia-smi query result")
    return {
        "name": row[0].strip(),
        "driver_version": row[1].strip(),
        "memory_total_mib": int(row[2]),
        "memory_used_mib": int(row[3]),
        "memory_free_mib": int(row[4]),
        "utilization_percent": int(row[5]),
        "temperature_c": int(row[6]),
    }


def build_checks(snapshot: dict[str, Any], model: str, vram_ceiling_mib: int) -> list[dict[str, Any]]:
    nodes = snapshot["kubernetes"]["nodes"]
    pods = snapshot["kubernetes"]["pods"]
    models = snapshot["ollama"]["models"]
    running = snapshot["ollama"]["running"]
    parameters = snapshot["ollama"]["parameters"]
    expected_name = lambda value: value == model or value == f"{model}:latest"
    resident = next((entry for entry in running if expected_name(entry["name"])), None)

    checks = [
        ("kubernetes-node-ready", bool(nodes) and all(node["ready"] for node in nodes), "all observed nodes are Ready"),
        ("nvidia-gpu-allocatable", any(node["gpu_allocatable"] >= 1 for node in nodes), "at least one nvidia.com/gpu is allocatable"),
        ("nvidia-runtimeclass", snapshot["kubernetes"]["nvidia_runtimeclass"], "RuntimeClass/nvidia exists"),
        ("helm-release-deployed", snapshot["helm"]["status"] == "deployed", "LLM stack Helm release is deployed"),
        ("workloads-ready", bool(pods) and all(pod["ready"] for pod in pods), "all observed application containers are Ready"),
        ("qwen-registered", any(expected_name(entry["name"]) for entry in models), "expected local Qwen alias is registered"),
        ("qwen-resident", resident is not None, "expected local Qwen alias is loaded"),
        ("qwen-gpu-active", bool(resident and "GPU" in resident["processor"]), "Ollama reports GPU participation"),
        ("qwen-keep-alive", bool(resident and resident["until"] == "Forever"), "Ollama reports Until=Forever"),
        ("vram-ceiling", snapshot["nvidia"]["memory_used_mib"] <= vram_ceiling_mib, f"GPU memory stays at or below {vram_ceiling_mib} MiB"),
        ("num-gpu-layers", parameters.get("num_gpu") == 23, "num_gpu is 23"),
        ("context-window", parameters.get("num_ctx") == 256, "num_ctx is 256"),
        ("batch-size", parameters.get("num_batch") == 1, "num_batch is 1"),
    ]
    return [{"name": name, "passed": passed, "detail": detail} for name, passed, detail in checks]


def collect_snapshot(
    runner: Runner,
    namespace: str,
    release: str,
    model: str,
    vram_ceiling_mib: int,
) -> dict[str, Any]:
    nodes_json = runner.json(["kubectl", "get", "nodes", "-o", "json"])
    pods_json = runner.json(["kubectl", "get", "pods", "-n", namespace, "-o", "json"])
    version_json = runner.json(["kubectl", "version", "-o", "json"])
    runtime_json = runner.json(["kubectl", "get", "runtimeclass", "nvidia", "-o", "json"])
    helm_json = runner.json(["helm", "status", release, "-n", namespace, "-o", "json"])

    pod_items = pods_json.get("items", [])
    ollama_pod = find_ollama_pod(pod_items)
    ollama = ["kubectl", "exec", "-n", namespace, ollama_pod, "--", "ollama"]
    list_output = runner.run([*ollama, "list"])
    ps_output = runner.run([*ollama, "ps"])
    params_output = runner.run([*ollama, "show", model, "--parameters"])

    nodes = []
    for index, item in enumerate(nodes_json.get("items", []), start=1):
        metadata = item.get("metadata", {})
        status = item.get("status", {})
        labels = metadata.get("labels", {})
        nodes.append(
            {
                "name": f"node-{index}",
                "ready": ready_condition(item),
                "os_image": status.get("nodeInfo", {}).get("osImage", "unknown"),
                "kernel": status.get("nodeInfo", {}).get("kernelVersion", "unknown"),
                "container_runtime": status.get("nodeInfo", {}).get("containerRuntimeVersion", "unknown"),
                "gpu_present": labels.get("nvidia.com/gpu.present") == "true",
                "gpu_capacity": int(status.get("capacity", {}).get("nvidia.com/gpu", 0)),
                "gpu_allocatable": int(status.get("allocatable", {}).get("nvidia.com/gpu", 0)),
            }
        )

    pods = []
    for item in pod_items:
        metadata = item.get("metadata", {})
        status = item.get("status", {})
        pods.append(
            {
                "name": metadata.get("name", "unknown"),
                "phase": status.get("phase", "Unknown"),
                "ready": pod_ready(item),
                "restarts": sum(s.get("restartCount", 0) for s in status.get("containerStatuses", [])),
            }
        )

    helm_status = helm_json.get("info", {}).get("status", helm_json.get("status", "unknown"))
    snapshot: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "observed_at": datetime.now(timezone.utc).replace(microsecond=0).isoformat(),
        "scope": {"namespace": namespace, "release": release, "model": model, "vram_ceiling_mib": vram_ceiling_mib},
        "host": {**parse_os_release(), "architecture": platform.machine(), "kernel": platform.release()},
        "nvidia": nvidia_snapshot(runner),
        "kubernetes": {
            "client_version": version_json.get("clientVersion", {}).get("gitVersion", "unknown"),
            "server_version": version_json.get("serverVersion", {}).get("gitVersion", "unknown"),
            "nvidia_runtimeclass": runtime_json.get("handler") == "nvidia",
            "nodes": nodes,
            "pods": pods,
        },
        "helm": {"name": helm_json.get("name", release), "namespace": helm_json.get("namespace", namespace), "status": helm_status},
        "ollama": {
            "pod": ollama_pod,
            "models": parse_ollama_list(list_output),
            "running": parse_ollama_ps(ps_output),
            "parameters": parse_parameters(params_output),
        },
    }
    snapshot["checks"] = build_checks(snapshot, model, vram_ceiling_mib)
    snapshot["summary"] = {
        "passed": sum(check["passed"] for check in snapshot["checks"]),
        "failed": sum(not check["passed"] for check in snapshot["checks"]),
    }
    return snapshot


def write_json(path: Path, value: dict[str, Any]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def render_validation(snapshot: dict[str, Any]) -> str:
    lines = []
    for check in snapshot["checks"]:
        marker = "PASS" if check["passed"] else "FAIL"
        lines.append(f"[{marker}] {check['name']}: {check['detail']}")
    summary = snapshot["summary"]
    lines.append(f"\n{summary['passed']} passed, {summary['failed']} failed")
    return "\n".join(lines)


def render_report(snapshot: dict[str, Any]) -> str:
    node = snapshot["kubernetes"]["nodes"][0]
    gpu = snapshot["nvidia"]
    running = snapshot["ollama"]["running"]
    resident = running[0] if running else {"processor": "not loaded", "context": "n/a", "until": "n/a"}
    checks = "\n".join(
        f"| {check['name']} | {'PASS' if check['passed'] else 'FAIL'} | {check['detail']} |"
        for check in snapshot["checks"]
    )
    return f"""# Live Qwen GGUF observation

Observed at `{snapshot['observed_at']}` using evidence schema `{snapshot['schema_version']}`.

## Runtime

| Field | Observed value |
|---|---|
| Host | {snapshot['host'].get('pretty_name', 'unknown')} |
| Kubernetes | {snapshot['kubernetes']['server_version']} |
| Node | {node['name']} (Ready: {str(node['ready']).lower()}) |
| GPU | {gpu['name']} ({gpu['memory_total_mib']} MiB) |
| GPU memory | {gpu['memory_used_mib']} MiB used / {gpu['memory_free_mib']} MiB free |
| Model | `{snapshot['scope']['model']}` |
| Processor split | {resident['processor']} |
| Context | {resident['context']} |
| Residency | {resident['until']} |
| Helm release | {snapshot['helm']['name']} ({snapshot['helm']['status']}) |

## Checks

| Check | Result | Contract |
|---|---|---|
{checks}

This report contains selected operational facts only. It excludes Secrets,
kubeconfig data, environment variables, pod logs, and model weights.
"""


def smoke(runner: Runner, namespace: str, model: str, prompt: str, expected: str | None) -> dict[str, Any]:
    pods_json = runner.json(["kubectl", "get", "pods", "-n", namespace, "-o", "json"])
    pod = find_ollama_pod(pods_json.get("items", []))
    started = time.monotonic()
    response = runner.run(["kubectl", "exec", "-n", namespace, pod, "--", "ollama", "run", model, prompt])
    duration = round(time.monotonic() - started, 3)
    passed = bool(response.strip()) and (expected is None or expected.lower() in response.lower())
    return {
        "observed_at": datetime.now(timezone.utc).replace(microsecond=0).isoformat(),
        "model": model,
        "prompt": prompt,
        "expected_substring": expected,
        "response": response.strip(),
        "duration_seconds": duration,
        "passed": passed,
    }


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    root.add_argument("--namespace", default=os.environ.get("QWEN_NAMESPACE", DEFAULT_NAMESPACE))
    root.add_argument("--release", default=os.environ.get("QWEN_RELEASE", DEFAULT_RELEASE))
    root.add_argument("--model", default=os.environ.get("QWEN_MODEL", DEFAULT_MODEL))
    root.add_argument("--vram-ceiling-mib", type=int, default=int(os.environ.get("QWEN_VRAM_CEILING_MIB", "850")))
    commands = root.add_subparsers(dest="command", required=True)
    validate = commands.add_parser("validate", help="run read-only runtime contract checks")
    validate.add_argument("--json", action="store_true", dest="as_json")
    snapshot = commands.add_parser("snapshot", help="write a sanitized JSON observation")
    snapshot.add_argument("--output", type=Path, required=True)
    report = commands.add_parser("report", help="render Markdown from a saved snapshot")
    report.add_argument("--input", type=Path, required=True)
    report.add_argument("--output", type=Path, required=True)
    smoke_parser = commands.add_parser("smoke", help="run one explicit Qwen inference check")
    smoke_parser.add_argument("--prompt", default="Reply with exactly: qwen observation ok")
    smoke_parser.add_argument("--expect", default="qwen observation ok")
    smoke_parser.add_argument("--output", type=Path)
    return root


def main(argv: Sequence[str] | None = None) -> int:
    args = parser().parse_args(argv)
    runner = Runner()
    try:
        if args.command in {"validate", "snapshot"}:
            snapshot = collect_snapshot(runner, args.namespace, args.release, args.model, args.vram_ceiling_mib)
            if args.command == "snapshot":
                write_json(args.output, snapshot)
                print(args.output)
            elif args.as_json:
                print(json.dumps(snapshot, indent=2, ensure_ascii=False))
            else:
                print(render_validation(snapshot))
            return 1 if snapshot["summary"]["failed"] else 0
        if args.command == "report":
            snapshot = json.loads(args.input.read_text(encoding="utf-8"))
            args.output.parent.mkdir(parents=True, exist_ok=True)
            args.output.write_text(render_report(snapshot), encoding="utf-8")
            print(args.output)
            return 0
        if args.command == "smoke":
            result = smoke(runner, args.namespace, args.model, args.prompt, args.expect)
            if args.output:
                write_json(args.output, result)
            print(result["response"])
            print(f"duration={result['duration_seconds']}s passed={str(result['passed']).lower()}")
            return 0 if result["passed"] else 1
    except (ObservationError, OSError, ValueError, json.JSONDecodeError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
