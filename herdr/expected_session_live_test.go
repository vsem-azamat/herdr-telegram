//go:build linux

package herdr_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/vsem-azamat/herdr-telegram/herdr"
)

func TestExpectedSessionForkLive(t *testing.T) {
	binary := os.Getenv("HERDR_EXPECTED_SESSION_BIN")
	if binary == "" {
		t.Skip("set HERDR_EXPECTED_SESSION_BIN to run the disposable fork contract probe")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatalf("resolve Herdr binary: %v", err)
	}
	if _, err := os.Stat(binary); err != nil {
		t.Fatalf("stat Herdr binary: %v", err)
	}
	script, err := exec.LookPath("script")
	if err != nil {
		t.Skip("util-linux script is required for the disposable PTY")
	}

	root := t.TempDir()
	configHome := filepath.Join(root, "config")
	runtimeDir := filepath.Join(root, "run")
	binDir := filepath.Join(root, "bin")
	for _, directory := range []string{filepath.Join(configHome, "herdr"), runtimeDir, binDir} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatalf("create disposable directory: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(configHome, "herdr", "config.toml"), []byte("onboarding = false\n"), 0o600); err != nil {
		t.Fatalf("write disposable config: %v", err)
	}
	inputLog := filepath.Join(root, "agent-input.log")
	fakePI := filepath.Join(binDir, "pi")
	fakeAgent := "#!/bin/sh\nprintf 'Working...\\n'\nwhile IFS= read -r line; do\n  printf '%s\\n' \"$line\" >> \"$HERDR_PROBE_INPUT_LOG\"\ndone\n"
	if err := os.WriteFile(fakePI, []byte(fakeAgent), 0o700); err != nil {
		t.Fatalf("write disposable agent: %v", err)
	}

	socketPath := filepath.Join(runtimeDir, "herdr.sock")
	command := exec.Command(script, "-qefc", shellQuote(binary)+" server", "/dev/null")
	command.Env = cleanHerdrEnvironment(os.Environ(), map[string]string{
		"XDG_CONFIG_HOME":       configHome,
		"XDG_RUNTIME_DIR":       runtimeDir,
		"HERDR_SOCKET_PATH":     socketPath,
		"HERDR_PROBE_INPUT_LOG": inputLog,
		"SHELL":                 "/bin/sh",
		"PATH":                  binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
	})
	command.Stdout = nil
	command.Stderr = nil
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatalf("start disposable Herdr: %v", err)
	}
	serverDone := make(chan error, 1)
	go func() { serverDone <- command.Wait() }()
	t.Cleanup(func() {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
		select {
		case <-serverDone:
		case <-time.After(3 * time.Second):
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
			<-serverDone
		}
	})
	waitForSocket(t, socketPath)

	client, err := herdr.NewClient(socketPath)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server, err := client.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if server.Capabilities == nil || !server.Capabilities.AgentPromptExpectedSession {
		t.Fatalf("expected-session capability absent: %#v", server.Capabilities)
	}

	targetPane := createWorkspace(t, socketPath, root, true)
	rawCall(t, socketPath, "pane.send_text", map[string]any{"pane_id": targetPane, "text": "pi"})
	rawCall(t, socketPath, "pane.send_keys", map[string]any{"pane_id": targetPane, "keys": []string{"Enter"}})
	waitForAgentKind(t, client, targetPane, "pi")

	sessionAPath := filepath.Join(root, "session-a.jsonl")
	sessionBPath := filepath.Join(root, "session-b.jsonl")
	reportSession(t, socketPath, targetPane, sessionAPath, "startup", 1)
	reportAgent(t, socketPath, targetPane, sessionAPath, "working", 2)
	sessionA := waitForAgentSession(t, client, targetPane, sessionAPath)

	_ = createWorkspace(t, socketPath, root, true)
	target, err := client.GetAgent(ctx, targetPane)
	if err != nil {
		t.Fatalf("GetAgent() error = %v", err)
	}
	if target.Focused {
		t.Fatal("target pane remained focused; probe must route independently of focus")
	}

	acceptedMarker := "accepted-for-session-a"
	accepted, err := client.Prompt(ctx, targetPane, acceptedMarker, herdr.PromptOptions{ExpectedSession: sessionA})
	if err != nil {
		t.Fatalf("matching Prompt() error = %v", err)
	}
	if accepted.AgentSession == nil || *accepted.AgentSession != *sessionA {
		t.Fatalf("matching Prompt().AgentSession = %#v, want session A", accepted.AgentSession)
	}
	waitForFileText(t, inputLog, acceptedMarker)

	waitMarker := "wait-pinned-to-session-a"
	waitTimeoutMS := uint64(3_000)
	waitResult := make(chan error, 1)
	go func() {
		waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer waitCancel()
		_, promptErr := client.Prompt(waitCtx, targetPane, waitMarker, herdr.PromptOptions{
			ExpectedSession: sessionA,
			Wait: &herdr.PromptWait{
				Until:     []herdr.AgentStatus{herdr.AgentStatusBlocked},
				TimeoutMS: &waitTimeoutMS,
			},
		})
		waitResult <- promptErr
	}()
	waitForFileText(t, inputLog, waitMarker)

	reportSession(t, socketPath, targetPane, sessionBPath, "new", 3)
	reportAgent(t, socketPath, targetPane, sessionBPath, "blocked", 4)
	_ = waitForAgentSession(t, client, targetPane, sessionBPath)
	select {
	case waitErr := <-waitResult:
		var waitAPIError *herdr.APIError
		if !errors.As(waitErr, &waitAPIError) || waitAPIError.Code != "agent_not_running" {
			t.Fatalf("session-replaced PromptWait() error = %v, want agent_not_running", waitErr)
		}
		var waitAmbiguous *herdr.AmbiguousPromptError
		if !errors.As(waitErr, &waitAmbiguous) {
			t.Fatalf("session-replaced PromptWait() error = %v, accepted prompt must remain ambiguous", waitErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("PromptWait did not stop after native session replacement")
	}

	inputBeforeRejection, err := os.ReadFile(inputLog)
	if err != nil {
		t.Fatalf("read disposable agent input before rejection: %v", err)
	}
	rejectedMarker := "must-not-reach-session-b"
	_, err = client.Prompt(ctx, targetPane, rejectedMarker, herdr.PromptOptions{ExpectedSession: sessionA})
	var apiErr *herdr.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != herdr.ErrorCodeAgentSessionMismatch {
		t.Fatalf("stale Prompt() error = %v, want agent_session_mismatch", err)
	}
	var ambiguous *herdr.AmbiguousPromptError
	if errors.As(err, &ambiguous) {
		t.Fatalf("stale Prompt() error = %v, want known rejection", err)
	}
	time.Sleep(300 * time.Millisecond)
	contents, readErr := os.ReadFile(inputLog)
	if readErr != nil {
		t.Fatalf("read disposable agent input: %v", readErr)
	}
	if bytes.Contains(contents, []byte(rejectedMarker)) {
		t.Fatal("stale prompt text reached the replacement session")
	}
	if !bytes.Equal(contents, inputBeforeRejection) {
		t.Fatal("stale prompt emitted terminal input after rejection")
	}

	t.Logf("redacted contract evidence: protocol=%d capability=true focus_independent=true matching_prompt=accepted wait_session_pinned=true replacement_prompt=agent_session_mismatch replacement_input=false", server.Protocol)
}

func createWorkspace(t *testing.T, socketPath, cwd string, focus bool) string {
	t.Helper()
	result := rawCall(t, socketPath, "workspace.create", map[string]any{"cwd": cwd, "focus": focus})
	var created struct {
		RootPane struct {
			PaneID string `json:"pane_id"`
		} `json:"root_pane"`
	}
	decodeRawResult(t, result, &created)
	if created.RootPane.PaneID == "" {
		t.Fatal("workspace.create returned an empty pane ID")
	}
	return created.RootPane.PaneID
}

func reportSession(t *testing.T, socketPath, paneID, sessionPath, startSource string, seq uint64) {
	t.Helper()
	rawCall(t, socketPath, "pane.report_agent_session", map[string]any{
		"pane_id": paneID, "source": "herdr:pi", "agent": "pi", "agent_session_path": sessionPath,
		"session_start_source": startSource, "seq": seq,
	})
}

func reportAgent(t *testing.T, socketPath, paneID, sessionPath, state string, seq uint64) {
	t.Helper()
	rawCall(t, socketPath, "pane.report_agent", map[string]any{
		"pane_id": paneID, "source": "herdr:pi", "agent": "pi", "state": state,
		"agent_session_path": sessionPath, "seq": seq,
	})
}

func waitForAgentKind(t *testing.T, client *herdr.Client, paneID, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		agent, err := client.GetAgent(ctx, paneID)
		cancel()
		if err == nil && agent.Agent != nil && *agent.Agent == want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("pane %s did not detect disposable %s agent", paneID, want)
}

func waitForAgentSession(t *testing.T, client *herdr.Client, paneID, wantPath string) *herdr.AgentSession {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		agent, err := client.GetAgent(ctx, paneID)
		cancel()
		if err == nil && agent.AgentSession != nil && agent.AgentSession.Value == wantPath {
			return agent.AgentSession
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("pane %s did not report expected disposable session", paneID)
	return nil
}

func waitForFileText(t *testing.T, path, text string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		contents, err := os.ReadFile(path)
		if err == nil && bytes.Contains(contents, []byte(text)) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("disposable agent did not receive accepted prompt")
}

type liveRawResponse struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *herdr.APIError `json:"error"`
}

func rawCall(t *testing.T, socketPath, method string, params any) json.RawMessage {
	t.Helper()
	connection, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		t.Fatalf("dial disposable Herdr: %v", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(3 * time.Second))
	requestID := "probe-" + strings.ReplaceAll(method, ".", "-")
	if err := json.NewEncoder(connection).Encode(map[string]any{"id": requestID, "method": method, "params": params}); err != nil {
		t.Fatalf("encode %s request: %v", method, err)
	}
	var response liveRawResponse
	if err := json.NewDecoder(bufio.NewReader(connection)).Decode(&response); err != nil {
		t.Fatalf("decode %s response: %v", method, err)
	}
	if response.Error != nil {
		t.Fatalf("%s API error: code=%s", method, response.Error.Code)
	}
	if response.ID != requestID {
		t.Fatalf("%s response ID = %q, want %q", method, response.ID, requestID)
	}
	if len(response.Result) == 0 {
		t.Fatalf("%s returned no result", method)
	}
	return response.Result
}

func decodeRawResult(t *testing.T, result json.RawMessage, destination any) {
	t.Helper()
	if err := json.Unmarshal(result, destination); err != nil {
		t.Fatalf("decode raw result: %v", err)
	}
}

func waitForSocket(t *testing.T, socketPath string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(socketPath); err == nil && info.Mode()&os.ModeSocket != 0 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("disposable Herdr socket was not created")
}

func cleanHerdrEnvironment(environment []string, additions map[string]string) []string {
	clean := make([]string, 0, len(environment)+len(additions))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		_, replaced := additions[name]
		if replaced || strings.HasPrefix(name, "HERDR_") || name == "XDG_CONFIG_HOME" || name == "XDG_RUNTIME_DIR" {
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
