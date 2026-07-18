package observer

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	SchemaVersion    = "1.0"
	DefaultNamespace = "llm-observability"
	DefaultRelease   = "llm-observability-stack"
	DefaultModel     = "qwen-1-8b-chat-q4-k-m-local"
)

type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func ExitCode(err error) int {
	var e *ExitError
	if errors.As(err, &e) {
		return e.Code
	}
	return 2
}

type Runner interface {
	Run(args ...string) (string, error)
}
type CommandRunner struct{}

func (CommandRunner) Run(args ...string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("empty command")
	}
	cmd := exec.Command(args[0], args[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("command failed (%s): %s: %w", strings.Join(args, " "), detail, err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

type Model struct {
	Name     string `json:"name"`
	ID       string `json:"id"`
	Size     string `json:"size"`
	Modified string `json:"modified"`
}
type RunningModel struct {
	Name      string `json:"name"`
	ID        string `json:"id"`
	Size      string `json:"size"`
	Processor string `json:"processor"`
	Context   string `json:"context"`
	Until     string `json:"until"`
}
type GPU struct {
	Name               string `json:"name"`
	DriverVersion      string `json:"driver_version"`
	MemoryTotalMiB     int    `json:"memory_total_mib"`
	MemoryUsedMiB      int    `json:"memory_used_mib"`
	MemoryFreeMiB      int    `json:"memory_free_mib"`
	UtilizationPercent int    `json:"utilization_percent"`
	TemperatureC       int    `json:"temperature_c"`
}
type Node struct {
	Name             string `json:"name"`
	Ready            bool   `json:"ready"`
	OSImage          string `json:"os_image"`
	Kernel           string `json:"kernel"`
	ContainerRuntime string `json:"container_runtime"`
	GPUPresent       bool   `json:"gpu_present"`
	GPUCapacity      int    `json:"gpu_capacity"`
	GPUAllocatable   int    `json:"gpu_allocatable"`
}
type Pod struct {
	Name     string `json:"name"`
	Phase    string `json:"phase"`
	Ready    bool   `json:"ready"`
	Restarts int    `json:"restarts"`
}
type Check struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail"`
}

type Snapshot struct {
	SchemaVersion string         `json:"schema_version"`
	ObservedAt    string         `json:"observed_at"`
	Scope         map[string]any `json:"scope"`
	Host          map[string]any `json:"host"`
	NVIDIA        GPU            `json:"nvidia"`
	Kubernetes    struct {
		ClientVersion      string `json:"client_version"`
		ServerVersion      string `json:"server_version"`
		NVIDIARuntimeClass bool   `json:"nvidia_runtimeclass"`
		Nodes              []Node `json:"nodes"`
		Pods               []Pod  `json:"pods"`
	} `json:"kubernetes"`
	Helm struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Status    string `json:"status"`
	} `json:"helm"`
	Ollama struct {
		Pod        string         `json:"pod"`
		Models     []Model        `json:"models"`
		Running    []RunningModel `json:"running"`
		Parameters map[string]any `json:"parameters"`
	} `json:"ollama"`
	Checks  []Check `json:"checks"`
	Summary struct {
		Passed int `json:"passed"`
		Failed int `json:"failed"`
	} `json:"summary"`
}

var tableSeparator = regexp.MustCompile(`\s{2,}`)

func splitTable(output string) [][]string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return nil
	}
	rows := make([][]string, 0, len(lines)-1)
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) != "" {
			rows = append(rows, tableSeparator.Split(strings.TrimSpace(line), -1))
		}
	}
	return rows
}
func ParseOllamaList(output string) []Model {
	var result []Model
	for _, f := range splitTable(output) {
		if len(f) >= 4 {
			result = append(result, Model{f[0], f[1], f[2], f[3]})
		}
	}
	return result
}
func ParseOllamaPS(output string) []RunningModel {
	var result []RunningModel
	for _, f := range splitTable(output) {
		if len(f) >= 6 {
			result = append(result, RunningModel{f[0], f[1], f[2], f[3], f[4], f[5]})
		}
	}
	return result
}
func ParseParameters(output string) map[string]any {
	result := map[string]any{}
	var stops []string
	for _, line := range strings.Split(output, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		key, raw := f[0], strings.Trim(strings.Join(f[1:], " "), `"`)
		if key == "stop" {
			stops = append(stops, raw)
			continue
		}
		if v, err := strconv.Atoi(raw); err == nil {
			result[key] = v
		} else if v, err := strconv.ParseFloat(raw, 64); err == nil {
			result[key] = v
		} else {
			result[key] = raw
		}
	}
	if len(stops) > 0 {
		result["stop"] = stops
	}
	return result
}

func expectedModel(value, model string) bool { return value == model || value == model+":latest" }
func parameterInt(parameters map[string]any, key string) int {
	switch value := parameters[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	default:
		return -1
	}
}
func BuildChecks(s *Snapshot, model string, ceiling int) []Check {
	nodesReady, podsReady, gpuAllocatable := len(s.Kubernetes.Nodes) > 0, len(s.Kubernetes.Pods) > 0, false
	for _, n := range s.Kubernetes.Nodes {
		nodesReady = nodesReady && n.Ready
		gpuAllocatable = gpuAllocatable || n.GPUAllocatable >= 1
	}
	for _, p := range s.Kubernetes.Pods {
		podsReady = podsReady && p.Ready
	}
	registered := false
	for _, m := range s.Ollama.Models {
		registered = registered || expectedModel(m.Name, model)
	}
	var resident *RunningModel
	for i := range s.Ollama.Running {
		if expectedModel(s.Ollama.Running[i].Name, model) {
			resident = &s.Ollama.Running[i]
			break
		}
	}
	return []Check{
		{"kubernetes-node-ready", nodesReady, "all observed nodes are Ready"},
		{"nvidia-gpu-allocatable", gpuAllocatable, "at least one nvidia.com/gpu is allocatable"},
		{"nvidia-runtimeclass", s.Kubernetes.NVIDIARuntimeClass, "RuntimeClass/nvidia exists"},
		{"helm-release-deployed", s.Helm.Status == "deployed", "LLM stack Helm release is deployed"},
		{"workloads-ready", podsReady, "all observed application containers are Ready"},
		{"qwen-registered", registered, "expected local Qwen alias is registered"},
		{"qwen-resident", resident != nil, "expected local Qwen alias is loaded"},
		{"qwen-gpu-active", resident != nil && strings.Contains(resident.Processor, "GPU"), "Ollama reports GPU participation"},
		{"qwen-keep-alive", resident != nil && resident.Until == "Forever", "Ollama reports Until=Forever"},
		{"vram-ceiling", s.NVIDIA.MemoryUsedMiB <= ceiling, fmt.Sprintf("GPU memory stays at or below %d MiB", ceiling)},
		{"num-gpu-layers", parameterInt(s.Ollama.Parameters, "num_gpu") == 23, "num_gpu is 23"},
		{"context-window", parameterInt(s.Ollama.Parameters, "num_ctx") == 256, "num_ctx is 256"},
		{"batch-size", parameterInt(s.Ollama.Parameters, "num_batch") == 1, "num_batch is 1"},
	}
}

func get(object map[string]any, path ...string) any {
	var current any = object
	for _, key := range path {
		next, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = next[key]
	}
	return current
}
func text(value any, fallback string) string {
	if s, ok := value.(string); ok && s != "" {
		return s
	}
	return fallback
}
func integer(value any) int {
	switch v := value.(type) {
	case string:
		n, _ := strconv.Atoi(v)
		return n
	case float64:
		return int(v)
	default:
		return 0
	}
}
func items(object map[string]any) []map[string]any {
	raw, _ := object["items"].([]any)
	result := make([]map[string]any, 0, len(raw))
	for _, value := range raw {
		if item, ok := value.(map[string]any); ok {
			result = append(result, item)
		}
	}
	return result
}
func commandJSON(r Runner, args ...string) (map[string]any, error) {
	output, err := r.Run(args...)
	if err != nil {
		return nil, err
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(output), &object); err != nil {
		return nil, fmt.Errorf("invalid JSON from %s: %w", strings.Join(args, " "), err)
	}
	return object, nil
}
func readyCondition(item map[string]any) bool {
	values, _ := get(item, "status", "conditions").([]any)
	for _, raw := range values {
		c, _ := raw.(map[string]any)
		if c["type"] == "Ready" && c["status"] == "True" {
			return true
		}
	}
	return false
}
func podReady(item map[string]any) (bool, int) {
	values, _ := get(item, "status", "containerStatuses").([]any)
	ready, restarts := len(values) > 0, 0
	for _, raw := range values {
		s, _ := raw.(map[string]any)
		r, _ := s["ready"].(bool)
		ready = ready && r
		restarts += integer(s["restartCount"])
	}
	return ready, restarts
}
func findOllamaPod(pods []map[string]any) (string, error) {
	for _, pod := range pods {
		name := text(get(pod, "metadata", "name"), "")
		label := text(get(pod, "metadata", "labels", "app.kubernetes.io/name"), "")
		if text(get(pod, "status", "phase"), "") == "Running" && (label == "ollama" || strings.HasPrefix(name, "ollama-")) {
			return name, nil
		}
	}
	return "", errors.New("no running Ollama pod found")
}
func parseOSRelease() map[string]any {
	result := map[string]any{}
	data, _ := os.ReadFile("/etc/os-release")
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && (key == "ID" || key == "VERSION_ID" || key == "PRETTY_NAME") {
			result[strings.ToLower(key)] = strings.Trim(value, `"`)
		}
	}
	return result
}
func nvidiaSnapshot(r Runner) (GPU, error) {
	output, err := r.Run("nvidia-smi", "--query-gpu=name,driver_version,memory.total,memory.used,memory.free,utilization.gpu,temperature.gpu", "--format=csv,noheader,nounits")
	if err != nil {
		return GPU{}, err
	}
	row, err := csv.NewReader(strings.NewReader(output)).Read()
	if err != nil || len(row) != 7 {
		return GPU{}, errors.New("unexpected nvidia-smi query result")
	}
	for i := range row {
		row[i] = strings.TrimSpace(row[i])
	}
	return GPU{row[0], row[1], integer(row[2]), integer(row[3]), integer(row[4]), integer(row[5]), integer(row[6])}, nil
}

func CollectSnapshot(r Runner, namespace, release, model string, ceiling int) (*Snapshot, error) {
	nodesJSON, err := commandJSON(r, "kubectl", "get", "nodes", "-o", "json")
	if err != nil {
		return nil, err
	}
	podsJSON, err := commandJSON(r, "kubectl", "get", "pods", "-n", namespace, "-o", "json")
	if err != nil {
		return nil, err
	}
	versionJSON, err := commandJSON(r, "kubectl", "version", "-o", "json")
	if err != nil {
		return nil, err
	}
	runtimeJSON, err := commandJSON(r, "kubectl", "get", "runtimeclass", "nvidia", "-o", "json")
	if err != nil {
		return nil, err
	}
	helmJSON, err := commandJSON(r, "helm", "status", release, "-n", namespace, "-o", "json")
	if err != nil {
		return nil, err
	}
	podItems := items(podsJSON)
	ollamaPod, err := findOllamaPod(podItems)
	if err != nil {
		return nil, err
	}
	base := []string{"kubectl", "exec", "-n", namespace, ollamaPod, "--", "ollama"}
	listOutput, err := r.Run(append(base, "list")...)
	if err != nil {
		return nil, err
	}
	psOutput, err := r.Run(append(base, "ps")...)
	if err != nil {
		return nil, err
	}
	parametersOutput, err := r.Run(append(base, "show", model, "--parameters")...)
	if err != nil {
		return nil, err
	}
	gpu, err := nvidiaSnapshot(r)
	if err != nil {
		return nil, err
	}
	s := &Snapshot{SchemaVersion: SchemaVersion, ObservedAt: time.Now().UTC().Truncate(time.Second).Format(time.RFC3339), Scope: map[string]any{"namespace": namespace, "release": release, "model": model, "vram_ceiling_mib": ceiling}, Host: parseOSRelease(), NVIDIA: gpu}
	s.Host["architecture"] = runtime.GOARCH
	if kernel, err := r.Run("uname", "-r"); err == nil {
		s.Host["kernel"] = kernel
	}
	s.Kubernetes.ClientVersion = text(get(versionJSON, "clientVersion", "gitVersion"), "unknown")
	s.Kubernetes.ServerVersion = text(get(versionJSON, "serverVersion", "gitVersion"), "unknown")
	s.Kubernetes.NVIDIARuntimeClass = text(runtimeJSON["handler"], "") == "nvidia"
	for i, item := range items(nodesJSON) {
		s.Kubernetes.Nodes = append(s.Kubernetes.Nodes, Node{fmt.Sprintf("node-%d", i+1), readyCondition(item), text(get(item, "status", "nodeInfo", "osImage"), "unknown"), text(get(item, "status", "nodeInfo", "kernelVersion"), "unknown"), text(get(item, "status", "nodeInfo", "containerRuntimeVersion"), "unknown"), text(get(item, "metadata", "labels", "nvidia.com/gpu.present"), "") == "true", integer(get(item, "status", "capacity", "nvidia.com/gpu")), integer(get(item, "status", "allocatable", "nvidia.com/gpu"))})
	}
	for _, item := range podItems {
		ready, restarts := podReady(item)
		s.Kubernetes.Pods = append(s.Kubernetes.Pods, Pod{text(get(item, "metadata", "name"), "unknown"), text(get(item, "status", "phase"), "Unknown"), ready, restarts})
	}
	s.Helm.Name = text(helmJSON["name"], release)
	s.Helm.Namespace = text(helmJSON["namespace"], namespace)
	s.Helm.Status = text(get(helmJSON, "info", "status"), text(helmJSON["status"], "unknown"))
	s.Ollama.Pod = ollamaPod
	s.Ollama.Models = ParseOllamaList(listOutput)
	s.Ollama.Running = ParseOllamaPS(psOutput)
	s.Ollama.Parameters = ParseParameters(parametersOutput)
	s.Checks = BuildChecks(s, model, ceiling)
	for _, check := range s.Checks {
		if check.Passed {
			s.Summary.Passed++
		} else {
			s.Summary.Failed++
		}
	}
	return s, nil
}

func WriteJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
func RenderValidation(s *Snapshot) string {
	var out strings.Builder
	for _, c := range s.Checks {
		marker := "FAIL"
		if c.Passed {
			marker = "PASS"
		}
		fmt.Fprintf(&out, "[%s] %s: %s\n", marker, c.Name, c.Detail)
	}
	fmt.Fprintf(&out, "\n%d passed, %d failed\n", s.Summary.Passed, s.Summary.Failed)
	return out.String()
}
func RenderReport(s *Snapshot) string {
	if len(s.Kubernetes.Nodes) == 0 {
		return "# Live Qwen GGUF observation\n\nNo nodes were observed.\n"
	}
	node := s.Kubernetes.Nodes[0]
	resident := RunningModel{Processor: "not loaded", Context: "n/a", Until: "n/a"}
	if len(s.Ollama.Running) > 0 {
		resident = s.Ollama.Running[0]
	}
	var checks strings.Builder
	for _, c := range s.Checks {
		result := "FAIL"
		if c.Passed {
			result = "PASS"
		}
		fmt.Fprintf(&checks, "| %s | %s | %s |\n", c.Name, result, c.Detail)
	}
	return fmt.Sprintf("# Live Qwen GGUF observation\n\nObserved at `%s` using evidence schema `%s`.\n\n## Runtime\n\n| Field | Observed value |\n|---|---|\n| Host | %s |\n| Kubernetes | %s |\n| Node | %s (Ready: %t) |\n| GPU | %s (%d MiB) |\n| GPU memory | %d MiB used / %d MiB free |\n| Model | `%v` |\n| Processor split | %s |\n| Context | %s |\n| Residency | %s |\n| Helm release | %s (%s) |\n\n## Checks\n\n| Check | Result | Contract |\n|---|---|---|\n%s\nThis report contains selected operational facts only. It excludes Secrets,\nkubeconfig data, environment variables, pod logs, and model weights.\n", s.ObservedAt, s.SchemaVersion, text(s.Host["pretty_name"], "unknown"), s.Kubernetes.ServerVersion, node.Name, node.Ready, s.NVIDIA.Name, s.NVIDIA.MemoryTotalMiB, s.NVIDIA.MemoryUsedMiB, s.NVIDIA.MemoryFreeMiB, s.Scope["model"], resident.Processor, resident.Context, resident.Until, s.Helm.Name, s.Helm.Status, checks.String())
}

type SmokeResult struct {
	ObservedAt        string  `json:"observed_at"`
	Model             string  `json:"model"`
	Prompt            string  `json:"prompt"`
	ExpectedSubstring string  `json:"expected_substring"`
	Response          string  `json:"response"`
	DurationSeconds   float64 `json:"duration_seconds"`
	Passed            bool    `json:"passed"`
}

func Smoke(r Runner, namespace, model, prompt, expected string) (*SmokeResult, error) {
	podsJSON, err := commandJSON(r, "kubectl", "get", "pods", "-n", namespace, "-o", "json")
	if err != nil {
		return nil, err
	}
	pod, err := findOllamaPod(items(podsJSON))
	if err != nil {
		return nil, err
	}
	started := time.Now()
	response, err := r.Run("kubectl", "exec", "-n", namespace, pod, "--", "ollama", "run", model, prompt)
	if err != nil {
		return nil, err
	}
	passed := strings.TrimSpace(response) != "" && (expected == "" || strings.Contains(strings.ToLower(response), strings.ToLower(expected)))
	return &SmokeResult{time.Now().UTC().Truncate(time.Second).Format(time.RFC3339), model, prompt, expected, strings.TrimSpace(response), float64(time.Since(started).Milliseconds()) / 1000, passed}, nil
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func RunCLI(args []string, stdout io.Writer) error {
	root := flag.NewFlagSet("qwen-observe", flag.ContinueOnError)
	root.SetOutput(io.Discard)
	namespace := root.String("namespace", env("QWEN_NAMESPACE", DefaultNamespace), "Kubernetes namespace")
	release := root.String("release", env("QWEN_RELEASE", DefaultRelease), "Helm release")
	model := root.String("model", env("QWEN_MODEL", DefaultModel), "Ollama model")
	ceilingDefault, _ := strconv.Atoi(env("QWEN_VRAM_CEILING_MIB", "850"))
	ceiling := root.Int("vram-ceiling-mib", ceilingDefault, "VRAM ceiling in MiB")
	if err := root.Parse(args); err != nil {
		return &ExitError{2, err}
	}
	remaining := root.Args()
	if len(remaining) == 0 {
		return &ExitError{2, errors.New("command required: validate, snapshot, report, or smoke")}
	}
	command, commandArgs, runner := remaining[0], remaining[1:], CommandRunner{}
	switch command {
	case "validate", "snapshot":
		flags := flag.NewFlagSet(command, flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		asJSON := flags.Bool("json", false, "print JSON")
		output := flags.String("output", "", "output path")
		if err := flags.Parse(commandArgs); err != nil {
			return &ExitError{2, err}
		}
		s, err := CollectSnapshot(runner, *namespace, *release, *model, *ceiling)
		if err != nil {
			return &ExitError{2, err}
		}
		if command == "snapshot" {
			if *output == "" {
				return &ExitError{2, errors.New("snapshot requires --output")}
			}
			if err := WriteJSON(*output, s); err != nil {
				return &ExitError{2, err}
			}
			fmt.Fprintln(stdout, *output)
		} else if *asJSON {
			data, _ := json.MarshalIndent(s, "", "  ")
			fmt.Fprintln(stdout, string(data))
		} else {
			fmt.Fprint(stdout, RenderValidation(s))
		}
		if s.Summary.Failed > 0 {
			return &ExitError{1, errors.New("runtime contract failed")}
		}
		return nil
	case "report":
		flags := flag.NewFlagSet(command, flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		input := flags.String("input", "", "snapshot path")
		output := flags.String("output", "", "report path")
		if err := flags.Parse(commandArgs); err != nil {
			return &ExitError{2, err}
		}
		if *input == "" || *output == "" {
			return &ExitError{2, errors.New("report requires --input and --output")}
		}
		data, err := os.ReadFile(*input)
		if err != nil {
			return &ExitError{2, err}
		}
		var s Snapshot
		if err := json.Unmarshal(data, &s); err != nil {
			return &ExitError{2, err}
		}
		if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
			return &ExitError{2, err}
		}
		if err := os.WriteFile(*output, []byte(RenderReport(&s)), 0o644); err != nil {
			return &ExitError{2, err}
		}
		fmt.Fprintln(stdout, *output)
		return nil
	case "smoke":
		flags := flag.NewFlagSet(command, flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		prompt := flags.String("prompt", "Reply with exactly: qwen observation ok", "prompt")
		expect := flags.String("expect", "qwen observation ok", "expected substring")
		output := flags.String("output", "", "optional output")
		if err := flags.Parse(commandArgs); err != nil {
			return &ExitError{2, err}
		}
		result, err := Smoke(runner, *namespace, *model, *prompt, *expect)
		if err != nil {
			return &ExitError{2, err}
		}
		if *output != "" {
			if err := WriteJSON(*output, result); err != nil {
				return &ExitError{2, err}
			}
		}
		fmt.Fprintf(stdout, "%s\nduration=%.3fs passed=%t\n", result.Response, result.DurationSeconds, result.Passed)
		if !result.Passed {
			return &ExitError{1, errors.New("smoke expectation failed")}
		}
		return nil
	default:
		return &ExitError{2, fmt.Errorf("unknown command %q", command)}
	}
}
