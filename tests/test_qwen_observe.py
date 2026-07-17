import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


MODULE_PATH = Path(__file__).parents[1] / "scripts" / "qwen_observe.py"
SPEC = importlib.util.spec_from_file_location("qwen_observe", MODULE_PATH)
qwen_observe = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(qwen_observe)


class ParsingTests(unittest.TestCase):
    def test_parse_ollama_list(self):
        output = "NAME  ID  SIZE  MODIFIED\nqwen-local:latest  abc123  1.2 GB  1 minute ago"
        self.assertEqual(qwen_observe.parse_ollama_list(output)[0]["name"], "qwen-local:latest")

    def test_parse_ollama_ps(self):
        output = "NAME  ID  SIZE  PROCESSOR  CONTEXT  UNTIL\nqwen-local:latest  abc  1.1 GB  27%/73% CPU/GPU  256  Forever"
        parsed = qwen_observe.parse_ollama_ps(output)[0]
        self.assertEqual(parsed["processor"], "27%/73% CPU/GPU")
        self.assertEqual(parsed["until"], "Forever")

    def test_parse_parameters(self):
        output = 'stop "<|im_start|>"\nnum_batch 1\nnum_ctx 256\nnum_gpu 23\ntemperature 0'
        parsed = qwen_observe.parse_parameters(output)
        self.assertEqual(parsed["stop"], ["<|im_start|>"])
        self.assertEqual(parsed["num_gpu"], 23)

    def test_find_ollama_pod(self):
        pods = [{"metadata": {"name": "ollama-abc", "labels": {"app.kubernetes.io/name": "ollama"}}, "status": {"phase": "Running"}}]
        self.assertEqual(qwen_observe.find_ollama_pod(pods), "ollama-abc")


class ContractTests(unittest.TestCase):
    def setUp(self):
        self.snapshot = {
            "nvidia": {"memory_used_mib": 824},
            "kubernetes": {
                "nvidia_runtimeclass": True,
                "nodes": [{"ready": True, "gpu_allocatable": 1}],
                "pods": [{"ready": True}],
            },
            "helm": {"status": "deployed"},
            "ollama": {
                "models": [{"name": "qwen-1-8b-chat-q4-k-m-local:latest"}],
                "running": [{"name": "qwen-1-8b-chat-q4-k-m-local:latest", "processor": "27%/73% CPU/GPU", "until": "Forever"}],
                "parameters": {"num_gpu": 23, "num_ctx": 256, "num_batch": 1},
            },
        }

    def test_expected_contract_passes(self):
        checks = qwen_observe.build_checks(self.snapshot, "qwen-1-8b-chat-q4-k-m-local", 850)
        self.assertTrue(all(check["passed"] for check in checks))

    def test_vram_regression_fails(self):
        self.snapshot["nvidia"]["memory_used_mib"] = 860
        checks = qwen_observe.build_checks(self.snapshot, "qwen-1-8b-chat-q4-k-m-local", 850)
        vram = next(check for check in checks if check["name"] == "vram-ceiling")
        self.assertFalse(vram["passed"])

    def test_json_writer_round_trip(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "snapshot.json"
            qwen_observe.write_json(path, self.snapshot)
            self.assertEqual(json.loads(path.read_text())["helm"]["status"], "deployed")


if __name__ == "__main__":
    unittest.main()
