//go:build linux

package spikes_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const (
	lifecycleHelperEnvironment = "HERDR_LIFECYCLE_HELPER"
	lifecyclePluginID          = "io.github.vsem-azamat.herdr-telegram.probe"
	lifecycleInstanceID        = "disposable-lifecycle-instance"
)

func TestPluginSystemdLifecycleLive(t *testing.T) {
	binary := os.Getenv("HERDR_PLUGIN_LIFECYCLE_BIN")
	if binary == "" {
		t.Skip("set HERDR_PLUGIN_LIFECYCLE_BIN to run the disposable lifecycle probe")
	}
	if _, err := exec.LookPath("systemd-run"); err != nil {
		t.Skip("systemd-run is required for the disposable lifecycle probe")
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		t.Skip("systemctl is required for the disposable lifecycle probe")
	}
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("util-linux script is required for the disposable Herdr PTY")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatalf("resolve Herdr binary: %v", err)
	}
	if _, err := os.Stat(binary); err != nil {
		t.Fatalf("stat Herdr binary: %v", err)
	}
	helperBinary, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test helper binary: %v", err)
	}

	root := t.TempDir()
	proveAtomicPublication(t, filepath.Join(root, "atomic-publication.json"))
	configHome := filepath.Join(root, "config")
	stateHome := filepath.Join(root, "state")
	runtimeDir := filepath.Join(root, "run")
	pluginRoot := filepath.Join(root, "plugin")
	for _, directory := range []string{filepath.Join(configHome, "herdr"), stateHome, runtimeDir, pluginRoot} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatalf("create disposable directory: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(configHome, "herdr", "config.toml"), []byte("onboarding = false\n"), 0o600); err != nil {
		t.Fatalf("write disposable Herdr config: %v", err)
	}

	manifest := fmt.Sprintf(`id = %q
name = "Lifecycle probe"
version = "0.0.0"
min_herdr_version = "0.7.0"
platforms = ["linux"]

[[startup]]
command = [%q, "-test.run=^TestPluginLifecycleHelperProcess$", "--", "register"]
`, lifecyclePluginID, helperBinary)
	if err := os.WriteFile(filepath.Join(pluginRoot, "herdr-plugin.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("write disposable plugin manifest: %v", err)
	}

	socketPath := filepath.Join(runtimeDir, "herdr.sock")
	herdrEnvironment := map[string]string{
		"XDG_CONFIG_HOME":          configHome,
		"XDG_STATE_HOME":           stateHome,
		"XDG_RUNTIME_DIR":          runtimeDir,
		"HERDR_SOCKET_PATH":        socketPath,
		lifecycleHelperEnvironment: "1",
		"SHELL":                    "/bin/sh",
	}
	runHerdrCLI(t, binary, herdrEnvironment, "plugin", "link", pluginRoot)

	pluginConfigDir := filepath.Join(configHome, "herdr", "plugins", "config", lifecyclePluginID)
	pluginStateDir := filepath.Join(stateHome, "herdr", "plugins", lifecyclePluginID)
	if err := os.WriteFile(filepath.Join(pluginConfigDir, "instance-id"), []byte(lifecycleInstanceID+"\n"), 0o600); err != nil {
		t.Fatalf("write disposable instance identity: %v", err)
	}
	descriptorPath := filepath.Join(pluginStateDir, "runtime.json")

	server := startDisposableHerdr(t, binary, herdrEnvironment)
	defer server.stop()
	firstDescriptor := waitForDescriptor(t, descriptorPath, "")
	assertDescriptorFileSecurity(t, descriptorPath)
	assertDescriptor(t, firstDescriptor, binary, socketPath, pluginConfigDir, pluginStateDir)

	unit := fmt.Sprintf("herdr-telegram-lifecycle-%d.service", os.Getpid())
	controlSocket := filepath.Join(runtimeDir, "companion.sock")
	readyPath := filepath.Join(runtimeDir, "companion.ready")
	startCountPath := filepath.Join(runtimeDir, "companion.starts")
	crashMarkerPath := filepath.Join(runtimeDir, "companion.crashed-once")
	mutationLogPath := filepath.Join(runtimeDir, "mutations.log")
	raceCheckedPath := filepath.Join(runtimeDir, "race.checked")
	raceReleasePath := filepath.Join(runtimeDir, "race.release")
	startCompanionService(t, unit, helperBinary, map[string]string{
		lifecycleHelperEnvironment: "1",
		"PROBE_DESCRIPTOR_PATH":    descriptorPath,
		"PROBE_EXPECTED_BINARY":    binary,
		"PROBE_EXPECTED_SOCKET":    socketPath,
		"PROBE_EXPECTED_CONFIG":    pluginConfigDir,
		"PROBE_EXPECTED_STATE":     pluginStateDir,
		"PROBE_CONTROL_SOCKET":     controlSocket,
		"PROBE_READY_PATH":         readyPath,
		"PROBE_START_COUNT_PATH":   startCountPath,
		"PROBE_CRASH_MARKER_PATH":  crashMarkerPath,
		"PROBE_MUTATION_LOG_PATH":  mutationLogPath,
		"PROBE_RACE_CHECKED_PATH":  raceCheckedPath,
		"PROBE_RACE_RELEASE_PATH":  raceReleasePath,
		"PROBE_PLUGIN_ID":          lifecyclePluginID,
		"PROBE_INSTANCE_ID":        lifecycleInstanceID,
	})
	defer stopCompanionService(unit)
	waitForPath(t, readyPath, 5*time.Second)
	waitForStartCount(t, startCountPath, 2)
	assertSystemdRestartPolicy(t, unit)
	proveSystemdStartLimit(t, fmt.Sprintf("herdr-telegram-lifecycle-limit-%d.service", os.Getpid()), helperBinary, runtimeDir)

	if got := companionRequest(t, controlSocket, "mutate"); got != "accepted" {
		t.Fatalf("enabled plugin mutation = %q, want accepted", got)
	}
	assertMutationCount(t, mutationLogPath, 1)

	// Coordinate the unresolved check-to-mutation race explicitly. The companion
	// has observed enabled state but pauses before its modeled side effect.
	type companionResult struct {
		value string
		err   error
	}
	raceResult := make(chan companionResult, 1)
	go func() {
		value, requestErr := companionRequestRaw(controlSocket, "mutate-race")
		raceResult <- companionResult{value: value, err: requestErr}
	}()
	waitForPath(t, raceCheckedPath, 3*time.Second)
	runHerdrCLI(t, binary, herdrEnvironment, "plugin", "disable", lifecyclePluginID)
	if err := os.WriteFile(raceReleasePath, []byte("release\n"), 0o600); err != nil {
		t.Fatalf("release coordinated mutation: %v", err)
	}
	select {
	case result := <-raceResult:
		if result.err != nil {
			t.Fatalf("coordinated mutation failed: %v", result.err)
		}
		if result.value != "accepted" {
			t.Fatalf("already-authorized mutation after disable = %q, want accepted evidence", result.value)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("coordinated mutation did not finish")
	}
	assertMutationCount(t, mutationLogPath, 2)

	if got := companionRequest(t, controlSocket, "mutate"); got != "plugin_disabled" {
		t.Fatalf("post-disable mutation = %q, want plugin_disabled", got)
	}
	assertMutationCount(t, mutationLogPath, 2)

	runHerdrCLI(t, binary, herdrEnvironment, "plugin", "enable", lifecyclePluginID)
	if got := companionRequest(t, controlSocket, "mutate"); got != "accepted" {
		t.Fatalf("re-enabled plugin mutation = %q, want accepted", got)
	}
	assertMutationCount(t, mutationLogPath, 3)

	runHerdrCLI(t, binary, herdrEnvironment, "plugin", "unlink", lifecyclePluginID)
	if got := companionRequest(t, controlSocket, "mutate"); got != "plugin_not_found" {
		t.Fatalf("post-unlink mutation = %q, want plugin_not_found", got)
	}
	assertMutationCount(t, mutationLogPath, 3)

	// Relinking does not execute a startup hook. Restarting the disposable Herdr
	// server does, and must replace the descriptor with a new process identity.
	runHerdrCLI(t, binary, herdrEnvironment, "plugin", "link", pluginRoot)
	oldDescriptorBytes, err := os.ReadFile(descriptorPath)
	if err != nil {
		t.Fatalf("read first runtime descriptor: %v", err)
	}
	server.stop()
	if got := companionRequest(t, controlSocket, "mutate"); got != "stale_descriptor" && got != "herdr_unavailable" {
		t.Fatalf("stopped-server mutation = %q, want stale_descriptor or herdr_unavailable", got)
	}
	assertMutationCount(t, mutationLogPath, 3)

	server = startDisposableHerdr(t, binary, herdrEnvironment)
	secondDescriptor := waitForDescriptor(t, descriptorPath, firstDescriptor.RegistrationNonce)
	assertDescriptorFileSecurity(t, descriptorPath)
	assertDescriptor(t, secondDescriptor, binary, socketPath, pluginConfigDir, pluginStateDir)
	if secondDescriptor.ServerPID == firstDescriptor.ServerPID && secondDescriptor.ServerStartTicks == firstDescriptor.ServerStartTicks {
		t.Fatal("Herdr restart did not refresh descriptor process identity")
	}
	if got := companionRequest(t, controlSocket, "mutate"); got != "accepted" {
		t.Fatalf("post-restart mutation = %q, want accepted", got)
	}
	assertMutationCount(t, mutationLogPath, 4)

	newDescriptorBytes, err := os.ReadFile(descriptorPath)
	if err != nil {
		t.Fatalf("read refreshed runtime descriptor: %v", err)
	}
	writeAtomicFile(t, descriptorPath, oldDescriptorBytes, 0o600)
	if got := companionRequest(t, controlSocket, "mutate"); got != "stale_descriptor" {
		t.Fatalf("replayed stale descriptor mutation = %q, want stale_descriptor", got)
	}
	assertMutationCount(t, mutationLogPath, 4)
	writeAtomicFile(t, descriptorPath, newDescriptorBytes, 0o600)

	if err := os.Chmod(descriptorPath, 0o644); err != nil {
		t.Fatalf("make descriptor intentionally insecure: %v", err)
	}
	if got := companionRequest(t, controlSocket, "mutate"); got != "descriptor_invalid" {
		t.Fatalf("insecure descriptor mutation = %q, want descriptor_invalid", got)
	}
	if err := os.Chmod(descriptorPath, 0o600); err != nil {
		t.Fatalf("restore descriptor permissions: %v", err)
	}
	oversizedDescriptor := append(append([]byte(nil), newDescriptorBytes...), bytes.Repeat([]byte(" "), 17<<10)...)
	writeAtomicFile(t, descriptorPath, oversizedDescriptor, 0o600)
	if got := companionRequest(t, controlSocket, "mutate"); got != "descriptor_invalid" {
		t.Fatalf("oversized descriptor mutation = %q, want descriptor_invalid", got)
	}
	writeAtomicFile(t, descriptorPath, newDescriptorBytes, 0o600)
	if got := companionRequest(t, controlSocket, "mutate"); got != "accepted" {
		t.Fatalf("restored descriptor mutation = %q, want accepted", got)
	}
	assertMutationCount(t, mutationLogPath, 5)

	t.Log("redacted lifecycle evidence: startup_descriptor=atomic restart_policy=bounded disable_after_return=denied unlink_after_return=denied restart_descriptor=refreshed stale_descriptor=denied insecure_descriptor=denied oversized_descriptor=denied disable_race=accepted_blocker")
}

// TestPluginLifecycleHelperProcess is executed as an ordinary subprocess by
// the disposable Herdr startup hook and transient systemd user service.
func TestPluginLifecycleHelperProcess(t *testing.T) {
	if os.Getenv(lifecycleHelperEnvironment) != "1" {
		t.Skip("helper subprocess only")
	}
	mode := helperMode(os.Args)
	var err error
	switch mode {
	case "register":
		err = registerRuntimeDescriptor()
	case "daemon":
		err = runCompanionProbe()
	default:
		err = fmt.Errorf("unknown lifecycle helper mode %q", mode)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

type runtimeDescriptor struct {
	Version           int    `json:"version"`
	PluginID          string `json:"plugin_id"`
	InstanceID        string `json:"instance_id"`
	SocketPath        string `json:"socket_path"`
	HerdrBinary       string `json:"herdr_binary"`
	HerdrVersion      string `json:"herdr_version"`
	Protocol          uint64 `json:"protocol"`
	PluginConfigDir   string `json:"plugin_config_dir"`
	PluginStateDir    string `json:"plugin_state_dir"`
	ServerPID         int    `json:"server_pid"`
	ServerStartTicks  string `json:"server_start_ticks"`
	RegistrationNonce string `json:"registration_nonce"`
}

func registerRuntimeDescriptor() error {
	configDir := os.Getenv("HERDR_PLUGIN_CONFIG_DIR")
	stateDir := os.Getenv("HERDR_PLUGIN_STATE_DIR")
	instanceBytes, err := os.ReadFile(filepath.Join(configDir, "instance-id"))
	if err != nil {
		return fmt.Errorf("read instance identity: %w", err)
	}
	instanceID := strings.TrimSpace(string(instanceBytes))
	if instanceID == "" {
		return errors.New("instance identity is empty")
	}
	serverPID := os.Getppid()
	startTicks, err := processStartTicks(serverPID)
	if err != nil {
		return fmt.Errorf("read server process identity: %w", err)
	}
	herdrVersion, protocol, err := readServerIdentity(os.Getenv("HERDR_SOCKET_PATH"))
	if err != nil {
		return fmt.Errorf("read server protocol identity: %w", err)
	}
	descriptor := runtimeDescriptor{
		Version: 1, PluginID: os.Getenv("HERDR_PLUGIN_ID"), InstanceID: instanceID,
		SocketPath: os.Getenv("HERDR_SOCKET_PATH"), HerdrBinary: os.Getenv("HERDR_BIN_PATH"),
		HerdrVersion: herdrVersion, Protocol: protocol,
		PluginConfigDir: configDir, PluginStateDir: stateDir, ServerPID: serverPID,
		ServerStartTicks:  startTicks,
		RegistrationNonce: fmt.Sprintf("%d-%d", serverPID, time.Now().UnixNano()),
	}
	encoded, err := json.Marshal(descriptor)
	if err != nil {
		return fmt.Errorf("encode runtime descriptor: %w", err)
	}
	return atomicWrite(filepath.Join(stateDir, "runtime.json"), append(encoded, '\n'), 0o600)
}

func runCompanionProbe() error {
	startCount, err := incrementCounter(os.Getenv("PROBE_START_COUNT_PATH"))
	if err != nil {
		return err
	}
	crashMarker := os.Getenv("PROBE_CRASH_MARKER_PATH")
	if os.Getenv("PROBE_FAIL_ALWAYS") == "1" {
		return errors.New("intentional persistent failure")
	}
	if startCount == 1 {
		if err := atomicWrite(crashMarker, []byte("intentional first-start failure\n"), 0o600); err != nil {
			return err
		}
		return errors.New("intentional first-start failure")
	}
	controlSocket := os.Getenv("PROBE_CONTROL_SOCKET")
	_ = os.Remove(controlSocket)
	listener, err := net.Listen("unix", controlSocket)
	if err != nil {
		return fmt.Errorf("listen on companion socket: %w", err)
	}
	defer listener.Close()
	defer os.Remove(controlSocket)
	if err := os.Chmod(controlSocket, 0o600); err != nil {
		return fmt.Errorf("restrict companion socket: %w", err)
	}
	if err := atomicWrite(os.Getenv("PROBE_READY_PATH"), []byte("ready\n"), 0o600); err != nil {
		return err
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)
	go func() {
		<-signals
		_ = listener.Close()
	}()
	for {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			if errors.Is(acceptErr, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept companion request: %w", acceptErr)
		}
		handleCompanionConnection(connection)
	}
}

func handleCompanionConnection(connection net.Conn) {
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
	request, err := bufio.NewReader(io.LimitReader(connection, 256)).ReadString('\n')
	if err != nil {
		_, _ = io.WriteString(connection, "invalid_request\n")
		return
	}
	request = strings.TrimSpace(request)
	result := authorizeCompanionMutation()
	if result == "accepted" && request == "mutate-race" {
		if err := atomicWrite(os.Getenv("PROBE_RACE_CHECKED_PATH"), []byte("checked\n"), 0o600); err != nil {
			result = "coordination_failed"
		} else {
			released := false
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				if _, statErr := os.Stat(os.Getenv("PROBE_RACE_RELEASE_PATH")); statErr == nil {
					released = true
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if !released {
				result = "coordination_timeout"
			}
		}
	}
	if result == "accepted" && (request == "mutate" || request == "mutate-race") {
		if err := appendMutationMarker(os.Getenv("PROBE_MUTATION_LOG_PATH")); err != nil {
			result = "local_write_failed"
		}
	} else if request != "mutate" && request != "mutate-race" {
		result = "invalid_request"
	}
	_, _ = io.WriteString(connection, result+"\n")
}

func authorizeCompanionMutation() string {
	descriptor, result := loadAndValidateDescriptor()
	if result != "" {
		return result
	}
	herdrVersion, protocol, err := readServerIdentity(descriptor.SocketPath)
	if err != nil || herdrVersion != descriptor.HerdrVersion || protocol != descriptor.Protocol {
		return "herdr_unavailable"
	}
	connection, err := net.DialTimeout("unix", descriptor.SocketPath, time.Second)
	if err != nil {
		return "herdr_unavailable"
	}
	defer connection.Close()
	if !unixPeerMatches(connection, descriptor.ServerPID, os.Getuid()) {
		return "herdr_unavailable"
	}
	_ = connection.SetDeadline(time.Now().Add(2 * time.Second))
	request := map[string]any{
		"id": "lifecycle-authorize", "method": "plugin.list",
		"params": map[string]any{"plugin_id": os.Getenv("PROBE_PLUGIN_ID")},
	}
	if err := json.NewEncoder(connection).Encode(request); err != nil {
		return "herdr_unavailable"
	}
	var response struct {
		ID     string `json:"id"`
		Result struct {
			Type    string `json:"type"`
			Plugins []struct {
				PluginID string `json:"plugin_id"`
				Enabled  bool   `json:"enabled"`
			} `json:"plugins"`
		} `json:"result"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.NewDecoder(bufio.NewReader(connection)).Decode(&response); err != nil || response.ID != "lifecycle-authorize" || len(response.Error) != 0 || response.Result.Type != "plugin_list" {
		return "herdr_unavailable"
	}
	if len(response.Result.Plugins) == 0 {
		return "plugin_not_found"
	}
	if len(response.Result.Plugins) != 1 || response.Result.Plugins[0].PluginID != os.Getenv("PROBE_PLUGIN_ID") {
		return "plugin_conflict"
	}
	if !response.Result.Plugins[0].Enabled {
		return "plugin_disabled"
	}
	return "accepted"
}

func loadAndValidateDescriptor() (runtimeDescriptor, string) {
	path := os.Getenv("PROBE_DESCRIPTOR_PATH")
	fileDescriptor, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return runtimeDescriptor{}, "descriptor_invalid"
	}
	file := os.NewFile(uintptr(fileDescriptor), path)
	if file == nil {
		_ = syscall.Close(fileDescriptor)
		return runtimeDescriptor{}, "descriptor_invalid"
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() > 16<<10 {
		return runtimeDescriptor{}, "descriptor_invalid"
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Getuid() {
		return runtimeDescriptor{}, "descriptor_invalid"
	}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var descriptor runtimeDescriptor
	if err := decoder.Decode(&descriptor); err != nil {
		return runtimeDescriptor{}, "descriptor_invalid"
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return runtimeDescriptor{}, "descriptor_invalid"
	}
	if descriptor.Version != 1 || descriptor.PluginID != os.Getenv("PROBE_PLUGIN_ID") || descriptor.InstanceID != os.Getenv("PROBE_INSTANCE_ID") || descriptor.SocketPath != os.Getenv("PROBE_EXPECTED_SOCKET") || descriptor.HerdrVersion == "" || descriptor.Protocol == 0 || descriptor.PluginConfigDir != os.Getenv("PROBE_EXPECTED_CONFIG") || descriptor.PluginStateDir != os.Getenv("PROBE_EXPECTED_STATE") {
		return runtimeDescriptor{}, "descriptor_invalid"
	}
	expectedBinary, err := filepath.EvalSymlinks(os.Getenv("PROBE_EXPECTED_BINARY"))
	if err != nil {
		return runtimeDescriptor{}, "descriptor_invalid"
	}
	descriptorBinary, err := filepath.EvalSymlinks(descriptor.HerdrBinary)
	if err != nil || descriptorBinary != expectedBinary {
		return runtimeDescriptor{}, "descriptor_invalid"
	}
	startTicks, err := processStartTicks(descriptor.ServerPID)
	if err != nil || startTicks != descriptor.ServerStartTicks {
		return runtimeDescriptor{}, "stale_descriptor"
	}
	socketInfo, err := os.Lstat(descriptor.SocketPath)
	if err != nil || socketInfo.Mode()&os.ModeSocket == 0 || socketInfo.Mode().Perm()&0o077 != 0 {
		return runtimeDescriptor{}, "herdr_unavailable"
	}
	socketStat, ok := socketInfo.Sys().(*syscall.Stat_t)
	if !ok || int(socketStat.Uid) != os.Getuid() {
		return runtimeDescriptor{}, "herdr_unavailable"
	}
	return descriptor, ""
}

func unixPeerMatches(connection net.Conn, expectedPID, expectedUID int) bool {
	unixConnection, ok := connection.(*net.UnixConn)
	if !ok {
		return false
	}
	raw, err := unixConnection.SyscallConn()
	if err != nil {
		return false
	}
	matched := false
	controlErr := raw.Control(func(fileDescriptor uintptr) {
		credentials, credentialsErr := syscall.GetsockoptUcred(int(fileDescriptor), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		if credentialsErr == nil {
			matched = int(credentials.Pid) == expectedPID && int(credentials.Uid) == expectedUID
		}
	})
	return controlErr == nil && matched
}

func readServerIdentity(socketPath string) (string, uint64, error) {
	connection, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		return "", 0, err
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(2 * time.Second))
	if err := json.NewEncoder(connection).Encode(map[string]any{
		"id": "lifecycle-ping", "method": "ping", "params": map[string]any{},
	}); err != nil {
		return "", 0, err
	}
	var response struct {
		ID     string `json:"id"`
		Result struct {
			Type     string `json:"type"`
			Version  string `json:"version"`
			Protocol uint64 `json:"protocol"`
		} `json:"result"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.NewDecoder(bufio.NewReader(connection)).Decode(&response); err != nil {
		return "", 0, err
	}
	if response.ID != "lifecycle-ping" || len(response.Error) != 0 || response.Result.Type != "pong" || response.Result.Version == "" || response.Result.Protocol == 0 {
		return "", 0, errors.New("invalid ping response")
	}
	return response.Result.Version, response.Result.Protocol, nil
}

func processStartTicks(pid int) (string, error) {
	contents, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", err
	}
	closing := bytes.LastIndexByte(contents, ')')
	if closing < 0 || closing+2 >= len(contents) {
		return "", errors.New("malformed process stat")
	}
	fields := strings.Fields(string(contents[closing+2:]))
	if len(fields) <= 19 {
		return "", errors.New("process stat lacks start time")
	}
	return fields[19], nil
}

func helperMode(arguments []string) string {
	for index, argument := range arguments {
		if argument == "--" && index+1 < len(arguments) {
			return arguments[index+1]
		}
	}
	return ""
}

func startDisposableHerdr(t *testing.T, binary string, additions map[string]string) *disposableProcess {
	t.Helper()
	script, err := exec.LookPath("script")
	if err != nil {
		t.Fatalf("find script: %v", err)
	}
	command := exec.Command(script, "-qefc", shellQuote(binary)+" server", "/dev/null")
	command.Env = cleanLifecycleEnvironment(os.Environ(), additions)
	command.Stdout = nil
	command.Stderr = nil
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatalf("start disposable Herdr: %v", err)
	}
	process := &disposableProcess{command: command, done: make(chan error, 1)}
	go func() { process.done <- command.Wait() }()
	t.Cleanup(process.stop)
	waitForSocket(t, additions["HERDR_SOCKET_PATH"], 5*time.Second)
	return process
}

type disposableProcess struct {
	command  *exec.Cmd
	done     chan error
	stopOnce sync.Once
}

func (process *disposableProcess) stop() {
	process.stopOnce.Do(func() {
		_ = syscall.Kill(-process.command.Process.Pid, syscall.SIGTERM)
		select {
		case <-process.done:
		case <-time.After(3 * time.Second):
			_ = syscall.Kill(-process.command.Process.Pid, syscall.SIGKILL)
			<-process.done
		}
	})
}

func runHerdrCLI(t *testing.T, binary string, additions map[string]string, arguments ...string) {
	t.Helper()
	command := exec.Command(binary, arguments...)
	command.Env = cleanLifecycleEnvironment(os.Environ(), additions)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("disposable Herdr %s failed: %v (output redacted, %d bytes)", strings.Join(arguments, " "), err, len(output))
	}
}

func proveAtomicPublication(t *testing.T, path string) {
	t.Helper()
	oldContents := append([]byte("old:"), bytes.Repeat([]byte("a"), 64<<10)...)
	newContents := append([]byte("new:"), bytes.Repeat([]byte("b"), 64<<10)...)
	if err := atomicWrite(path, oldContents, 0o600); err != nil {
		t.Fatalf("seed atomic publication probe: %v", err)
	}
	type readerResult struct {
		observations int
		err          error
	}
	stop := make(chan struct{})
	result := make(chan readerResult, 1)
	go func() {
		observations := 0
		for {
			select {
			case <-stop:
				result <- readerResult{observations: observations}
				return
			default:
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				result <- readerResult{observations: observations, err: fmt.Errorf("read publication: %w", err)}
				return
			}
			if !bytes.Equal(contents, oldContents) && !bytes.Equal(contents, newContents) {
				result <- readerResult{observations: observations, err: errors.New("reader observed partial or mixed publication")}
				return
			}
			observations++
		}
	}()
	for index := 0; index < 100; index++ {
		contents := oldContents
		if index%2 == 0 {
			contents = newContents
		}
		if err := atomicWrite(path, contents, 0o600); err != nil {
			close(stop)
			<-result
			t.Fatalf("exercise atomic publication: %v", err)
		}
	}
	close(stop)
	reader := <-result
	if reader.err != nil {
		t.Fatal(reader.err)
	}
	if reader.observations == 0 {
		t.Fatal("atomic publication reader made no observations")
	}
}

func startCompanionService(t *testing.T, unit, helperBinary string, environment map[string]string) {
	t.Helper()
	arguments := []string{
		"--user", "--unit=" + unit, "--collect",
		"--property=Type=simple", "--property=Restart=on-failure", "--property=RestartSec=100ms",
		"--property=StartLimitIntervalSec=10s", "--property=StartLimitBurst=3",
	}
	for name, value := range environment {
		arguments = append(arguments, "--setenv="+name+"="+value)
	}
	arguments = append(arguments, helperBinary, "-test.run=^TestPluginLifecycleHelperProcess$", "--", "daemon")
	command := exec.Command("systemd-run", arguments...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("start transient companion service: %v (output redacted, %d bytes)", err, len(output))
	}
}

func proveSystemdStartLimit(t *testing.T, unit, helperBinary, runtimeDir string) {
	t.Helper()
	startCountPath := filepath.Join(runtimeDir, "persistent-failure.starts")
	arguments := []string{
		"--user", "--unit=" + unit,
		"--property=Type=simple", "--property=Restart=on-failure", "--property=RestartSec=100ms",
		"--property=StartLimitIntervalSec=10s", "--property=StartLimitBurst=3",
		"--setenv=" + lifecycleHelperEnvironment + "=1",
		"--setenv=PROBE_FAIL_ALWAYS=1",
		"--setenv=PROBE_START_COUNT_PATH=" + startCountPath,
		helperBinary, "-test.run=^TestPluginLifecycleHelperProcess$", "--", "daemon",
	}
	if output, err := exec.Command("systemd-run", arguments...).CombinedOutput(); err != nil {
		t.Fatalf("start persistently failing transient service: %v (output redacted, %d bytes)", err, len(output))
	}
	defer stopCompanionService(unit)
	waitForStartCount(t, startCountPath, 3)

	deadline := time.Now().Add(5 * time.Second)
	bounded := false
	lastProperties := ""
	for time.Now().Before(deadline) {
		output, err := exec.Command("systemctl", "--user", "show", unit,
			"-p", "ActiveState", "-p", "Result", "-p", "NRestarts",
			"-p", "StartLimitBurst", "-p", "StartLimitIntervalUSec").Output()
		if err == nil {
			properties := string(output)
			lastProperties = properties
			bounded = strings.Contains(properties, "ActiveState=failed") &&
				strings.Contains(properties, "Result=start-limit-hit") &&
				strings.Contains(properties, "NRestarts=3") &&
				strings.Contains(properties, "StartLimitBurst=3") &&
				strings.Contains(properties, "StartLimitIntervalUSec=10s")
			if bounded {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !bounded {
		t.Fatalf("systemd did not stop the persistently failing service at the configured start limit; properties=%q", lastProperties)
	}
	contents, err := os.ReadFile(startCountPath)
	if err != nil {
		t.Fatalf("read bounded restart count: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	after, err := os.ReadFile(startCountPath)
	if err != nil {
		t.Fatalf("re-read bounded restart count: %v", err)
	}
	if !bytes.Equal(contents, after) {
		t.Fatal("persistently failing service restarted after the start limit was reached")
	}
}

func stopCompanionService(unit string) {
	_ = exec.Command("systemctl", "--user", "stop", unit).Run()
	_ = exec.Command("systemctl", "--user", "reset-failed", unit).Run()
}

func assertSystemdRestartPolicy(t *testing.T, unit string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		command := exec.Command("systemctl", "--user", "show", unit, "-p", "NRestarts", "-p", "Restart", "-p", "RestartUSec", "-p", "StartLimitBurst")
		output, err := command.Output()
		if err == nil {
			properties := string(output)
			if strings.Contains(properties, "NRestarts=1") && strings.Contains(properties, "Restart=on-failure") && strings.Contains(properties, "RestartUSec=100ms") && strings.Contains(properties, "StartLimitBurst=3") {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("transient systemd service did not expose the bounded restart policy")
}

func companionRequest(t *testing.T, socketPath, request string) string {
	t.Helper()
	response, err := companionRequestRaw(socketPath, request)
	if err != nil {
		t.Fatalf("disposable companion request: %v", err)
	}
	return response
}

func companionRequestRaw(socketPath, request string) (string, error) {
	connection, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		return "", fmt.Errorf("dial: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.WriteString(connection, request+"\n"); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}
	response, err := bufio.NewReader(io.LimitReader(connection, 256)).ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	return strings.TrimSpace(response), nil
}

func waitForDescriptor(t *testing.T, path, previousNonce string) runtimeDescriptor {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		contents, err := os.ReadFile(path)
		if err == nil {
			var descriptor runtimeDescriptor
			if json.Unmarshal(contents, &descriptor) == nil && descriptor.RegistrationNonce != "" && descriptor.RegistrationNonce != previousNonce {
				return descriptor
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("startup hook did not atomically publish a fresh runtime descriptor")
	return runtimeDescriptor{}
}

func assertDescriptorFileSecurity(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat runtime descriptor: %v", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 {
		t.Fatalf("runtime descriptor mode = %v, want regular 0600", info.Mode())
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Getuid() {
		t.Fatal("runtime descriptor is not owned by the current user")
	}
}

func assertDescriptor(t *testing.T, descriptor runtimeDescriptor, binary, socketPath, configDir, stateDir string) {
	t.Helper()
	if descriptor.Version != 1 || descriptor.PluginID != lifecyclePluginID || descriptor.InstanceID != lifecycleInstanceID || descriptor.SocketPath != socketPath || descriptor.HerdrVersion == "" || descriptor.Protocol == 0 || descriptor.PluginConfigDir != configDir || descriptor.PluginStateDir != stateDir || descriptor.RegistrationNonce == "" {
		t.Fatal("runtime descriptor fields do not match the disposable registration")
	}
	expectedBinary, err := filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatalf("resolve expected Herdr binary: %v", err)
	}
	actualBinary, err := filepath.EvalSymlinks(descriptor.HerdrBinary)
	if err != nil || actualBinary != expectedBinary {
		t.Fatal("runtime descriptor identifies a different Herdr binary")
	}
	startTicks, err := processStartTicks(descriptor.ServerPID)
	if err != nil || startTicks != descriptor.ServerStartTicks {
		t.Fatal("runtime descriptor process identity is stale")
	}
}

func assertMutationCount(t *testing.T, path string, want int) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read disposable mutation log: %v", err)
	}
	got := len(strings.Fields(string(contents)))
	if got != want {
		t.Fatalf("disposable mutation count = %d, want %d", got, want)
	}
}

func appendMutationMarker(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString("accepted\n")
	return err
}

func incrementCounter(path string) (int, error) {
	count := 0
	if contents, err := os.ReadFile(path); err == nil {
		count, _ = strconv.Atoi(strings.TrimSpace(string(contents)))
	} else if !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}
	count++
	return count, atomicWrite(path, []byte(strconv.Itoa(count)+"\n"), 0o600)
}

func waitForStartCount(t *testing.T, path string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		contents, err := os.ReadFile(path)
		if err == nil {
			count, _ := strconv.Atoi(strings.TrimSpace(string(contents)))
			if count >= want {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("systemd did not restart the disposable companion %d times", want)
}

func waitForSocket(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil && info.Mode()&os.ModeSocket != 0 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("disposable socket was not created")
}

func waitForPath(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("disposable path was not created")
}

func writeAtomicFile(t *testing.T, path string, contents []byte, mode os.FileMode) {
	t.Helper()
	if err := atomicWrite(path, contents, mode); err != nil {
		t.Fatalf("atomically write disposable file: %v", err)
	}
}

func atomicWrite(path string, contents []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".runtime-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func cleanLifecycleEnvironment(environment []string, additions map[string]string) []string {
	clean := make([]string, 0, len(environment)+len(additions))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		_, replaced := additions[name]
		if replaced || strings.HasPrefix(name, "HERDR_") || name == "XDG_CONFIG_HOME" || name == "XDG_STATE_HOME" || name == "XDG_RUNTIME_DIR" {
			continue
		}
		clean = append(clean, entry)
	}
	for name, value := range additions {
		clean = append(clean, name+"="+value)
	}
	return clean
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
