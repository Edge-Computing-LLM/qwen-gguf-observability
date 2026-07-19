package observer

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func fixture() *Snapshot {
	s := &Snapshot{NVIDIA: GPU{MemoryUsedMiB: 824}}
	s.Kubernetes.NVIDIARuntimeClass = true
	s.Kubernetes.Nodes = []Node{{Ready: true, GPUAllocatable: 1}}
	s.Kubernetes.Pods = []Pod{{Ready: true}}
	s.Helm.Status = "deployed"
	s.Ollama.Models = []Model{{Name: DefaultModel + ":latest"}}
	s.Ollama.Running = []RunningModel{{Name: DefaultModel + ":latest", Processor: "27%/73% CPU/GPU", Until: "Forever"}}
	s.Ollama.Parameters = map[string]any{"num_gpu": 23, "num_ctx": 256, "num_batch": 1}
	return s
}
func TestParseOllamaTables(t *testing.T) {
	models := ParseOllamaList("NAME  ID  SIZE  MODIFIED\nqwen-local:latest  abc123  1.2 GB  1 minute ago")
	if len(models) != 1 || models[0].Name != "qwen-local:latest" {
		t.Fatalf("models: %#v", models)
	}
	running := ParseOllamaPS("NAME  ID  SIZE  PROCESSOR  CONTEXT  UNTIL\nqwen-local:latest  abc  1.1 GB  27%/73% CPU/GPU  256  Forever")
	if len(running) != 1 || running[0].Processor != "27%/73% CPU/GPU" || running[0].Until != "Forever" {
		t.Fatalf("running: %#v", running)
	}
}
func TestParseParameters(t *testing.T) {
	p := ParseParameters("stop \"<|im_start|>\"\nnum_batch 1\nnum_ctx 256\nnum_gpu 23\ntemperature 0")
	if p["num_gpu"] != 23 || p["stop"].([]string)[0] != "<|im_start|>" {
		t.Fatalf("parameters: %#v", p)
	}
}
func TestContract(t *testing.T) {
	for _, c := range BuildChecks(fixture(), DefaultModel, 850) {
		if !c.Passed {
			t.Errorf("expected %s to pass", c.Name)
		}
	}
	s := fixture()
	s.NVIDIA.MemoryUsedMiB = 860
	for _, c := range BuildChecks(s, DefaultModel, 850) {
		if c.Name == "vram-ceiling" && c.Passed {
			t.Fatal("expected VRAM check to fail")
		}
	}
}
func TestJSONRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.json")
	if err := WriteJSON(path, fixture()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Snapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.NVIDIA.MemoryUsedMiB != 824 {
		t.Fatalf("snapshot: %#v", decoded)
	}
}

func TestSmokeJSONExcludesPromptAndResponse(t *testing.T) {
	result := SmokeResult{ObservedAt: "2026-07-19T00:00:00Z", Model: DefaultModel, DurationSeconds: 1.25, Passed: true, Response: "private response"}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"private response", "prompt", "response", "expected_substring"} {
		if bytes.Contains(data, []byte(forbidden)) {
			t.Fatalf("smoke JSON contains %q: %s", forbidden, data)
		}
	}
}

func TestHelpIsSuccessful(t *testing.T) {
	var output bytes.Buffer
	if err := RunCLI([]string{"--help"}, &output); err != nil {
		t.Fatalf("help failed: %v", err)
	}
	if !bytes.Contains(output.Bytes(), []byte("read-only Qwen GGUF")) {
		t.Fatalf("unexpected help output: %s", output.String())
	}
}

func TestReportSelectsExpectedRunningModel(t *testing.T) {
	s := fixture()
	s.Scope = map[string]any{"model": DefaultModel}
	s.Ollama.Running = append([]RunningModel{{Name: "other-model", Processor: "CPU", Context: "1", Until: "5m"}}, s.Ollama.Running...)
	report := RenderReport(s)
	if !bytes.Contains([]byte(report), []byte("27%/73% CPU/GPU")) {
		t.Fatalf("report selected the wrong running model:\n%s", report)
	}
}
