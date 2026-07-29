package herdr_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/vsem-azamat/herdr-telegram/herdr"
)

func TestNewClientRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		socketPath string
		options    []herdr.Option
	}{
		{name: "empty socket", socketPath: "  "},
		{name: "zero message limit", socketPath: "/tmp/herdr.sock", options: []herdr.Option{herdr.WithMaxMessageBytes(0)}},
		{name: "overflowing message limit", socketPath: "/tmp/herdr.sock", options: []herdr.Option{herdr.WithMaxMessageBytes(math.MaxInt64)}},
		{name: "nil option", socketPath: "/tmp/herdr.sock", options: []herdr.Option{nil}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := herdr.NewClient(test.socketPath, test.options...); err == nil {
				t.Fatal("NewClient() error = nil, want configuration error")
			}
		})
	}
}

func TestClientPingReturnsTypedAPIError(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "herdr.sock")
	serveOne(t, socketPath, func(t *testing.T, request map[string]json.RawMessage) any {
		return map[string]any{
			"id": requestID(t, request),
			"error": map[string]any{
				"code":    "protocol_mismatch",
				"message": "unsupported protocol",
			},
		}
	})

	client, err := herdr.NewClient(socketPath)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.Ping(context.Background())
	var apiErr *herdr.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("Ping() error = %v, want *herdr.APIError", err)
	}
	if apiErr.Code != "protocol_mismatch" {
		t.Errorf("APIError.Code = %q, want %q", apiErr.Code, "protocol_mismatch")
	}
	if apiErr.Message != "unsupported protocol" {
		t.Errorf("APIError.Message = %q, want %q", apiErr.Message, "unsupported protocol")
	}
}

func TestClientPing(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "herdr.sock")
	serveOne(t, socketPath, func(t *testing.T, request map[string]json.RawMessage) any {
		assertMethod(t, request, "ping")
		assertEmptyParams(t, request)

		return map[string]any{
			"id": requestID(t, request),
			"result": map[string]any{
				"type":     "pong",
				"version":  "0.7.5",
				"protocol": 17,
				"capabilities": map[string]any{
					"live_handoff":           true,
					"detached_server_daemon": true,
				},
			},
		}
	})

	client, err := herdr.NewClient(socketPath)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	got, err := client.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if got.Version != "0.7.5" {
		t.Errorf("Ping().Version = %q, want %q", got.Version, "0.7.5")
	}
	if got.Protocol != 17 {
		t.Errorf("Ping().Protocol = %d, want 17", got.Protocol)
	}
	if got.Capabilities == nil || !got.Capabilities.LiveHandoff {
		t.Errorf("Ping().Capabilities = %#v, want live handoff", got.Capabilities)
	}
	if got.Capabilities == nil || !got.Capabilities.DetachedServerDaemon {
		t.Errorf("Ping().Capabilities = %#v, want detached server daemon", got.Capabilities)
	}
}

func TestClientSnapshotDecodesStableAgentSession(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "herdr.sock")
	serveOne(t, socketPath, func(t *testing.T, request map[string]json.RawMessage) any {
		assertMethod(t, request, "session.snapshot")
		assertEmptyParams(t, request)

		return map[string]any{
			"id": requestID(t, request),
			"result": map[string]any{
				"type": "session_snapshot",
				"snapshot": map[string]any{
					"version":              "0.7.5",
					"protocol":             17,
					"focused_workspace_id": "w1",
					"focused_tab_id":       "w1:t1",
					"focused_pane_id":      "w1:p1",
					"workspaces": []any{
						map[string]any{
							"workspace_id": "w1", "number": 1, "label": "sdk", "focused": true, "pane_count": 1, "tab_count": 1, "active_tab_id": "w1:t1", "agent_status": "idle",
							"worktree": map[string]any{"repo_key": "repo-1", "repo_name": "sdk", "repo_root": "/repo", "checkout_path": "/repo/sdk", "is_linked_worktree": true},
						},
					},
					"tabs": []any{
						map[string]any{"tab_id": "w1:t1", "workspace_id": "w1", "number": 1, "label": "main", "focused": true, "pane_count": 1, "agent_status": "idle"},
					},
					"panes": []any{
						map[string]any{"pane_id": "w1:p1", "terminal_id": "term_1", "workspace_id": "w1", "tab_id": "w1:t1", "focused": true, "agent_status": "idle", "revision": 4},
					},
					"layouts": []any{},
					"agents": []any{
						map[string]any{
							"terminal_id": "term_1", "agent": "codex", "agent_status": "idle", "workspace_id": "w1", "tab_id": "w1:t1", "pane_id": "w1:p1", "focused": true, "state_change_seq": 9, "revision": 4,
							"agent_session": map[string]any{"source": "herdr:codex", "agent": "codex", "kind": "id", "value": "session-redacted"},
							"future_field":  "ignored",
						},
					},
				},
			},
		}
	})

	client, err := herdr.NewClient(socketPath)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	got, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if got.Version != "0.7.5" || got.Protocol != 17 {
		t.Fatalf("Snapshot() version/protocol = %s/%d, want 0.7.5/17", got.Version, got.Protocol)
	}
	if got.FocusedPaneID == nil || *got.FocusedPaneID != "w1:p1" {
		t.Errorf("Snapshot().FocusedPaneID = %v, want %q", got.FocusedPaneID, "w1:p1")
	}
	if got.Workspaces[0].Worktree == nil || got.Workspaces[0].Worktree.CheckoutPath != "/repo/sdk" || !got.Workspaces[0].Worktree.IsLinkedWorktree {
		t.Errorf("Snapshot().Workspaces[0].Worktree = %#v", got.Workspaces[0].Worktree)
	}
	if len(got.Agents) != 1 || got.Agents[0].AgentSession == nil {
		t.Fatalf("Snapshot().Agents = %#v, want one agent with session", got.Agents)
	}
	session := got.Agents[0].AgentSession
	if session.Source != "herdr:codex" || session.Agent != "codex" || session.Kind != herdr.AgentSessionID || session.Value != "session-redacted" {
		t.Errorf("Snapshot() agent session = %#v", session)
	}
}

func TestClientListAgents(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "herdr.sock")
	serveOne(t, socketPath, func(t *testing.T, request map[string]json.RawMessage) any {
		assertMethod(t, request, "agent.list")
		assertEmptyParams(t, request)
		return map[string]any{
			"id": requestID(t, request),
			"result": map[string]any{
				"type": "agent_list",
				"agents": []any{
					map[string]any{"terminal_id": "term_1", "agent": "claude", "agent_status": "working", "workspace_id": "w1", "tab_id": "w1:t1", "pane_id": "w1:p1", "focused": false, "state_change_seq": 11, "revision": 3},
				},
			},
		}
	})

	client, err := herdr.NewClient(socketPath)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	got, err := client.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("ListAgents() error = %v", err)
	}
	if len(got) != 1 || got[0].PaneID != "w1:p1" || got[0].Status != herdr.AgentStatusWorking {
		t.Fatalf("ListAgents() = %#v", got)
	}
}

func TestClientGetAgentTargetsPane(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "herdr.sock")
	serveOne(t, socketPath, func(t *testing.T, request map[string]json.RawMessage) any {
		assertMethod(t, request, "agent.get")
		var params struct {
			Target string `json:"target"`
		}
		decodeParams(t, request, &params)
		if params.Target != "w4:p7" {
			t.Errorf("target = %q, want %q", params.Target, "w4:p7")
		}
		return map[string]any{
			"id": requestID(t, request),
			"result": map[string]any{
				"type":  "agent_info",
				"agent": map[string]any{"terminal_id": "term_7", "agent": "codex", "agent_status": "idle", "workspace_id": "w4", "tab_id": "w4:t1", "pane_id": "w4:p7", "focused": false, "state_change_seq": 2, "revision": 1},
			},
		}
	})

	client, err := herdr.NewClient(socketPath)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	got, err := client.GetAgent(context.Background(), "w4:p7")
	if err != nil {
		t.Fatalf("GetAgent() error = %v", err)
	}
	if got.PaneID != "w4:p7" || got.Agent == nil || *got.Agent != "codex" {
		t.Fatalf("GetAgent() = %#v", got)
	}
}

func TestClientPromptEncodesWaitOptions(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "herdr.sock")
	serveOne(t, socketPath, func(t *testing.T, request map[string]json.RawMessage) any {
		assertMethod(t, request, "agent.prompt")
		var params struct {
			Target string `json:"target"`
			Text   string `json:"text"`
			Wait   struct {
				Until     []herdr.AgentStatus `json:"until"`
				TimeoutMS *uint64             `json:"timeout_ms"`
			} `json:"wait"`
		}
		decodeParams(t, request, &params)
		if params.Target != "w2:p4" || params.Text != "review this" {
			t.Errorf("prompt target/text = %q/%q", params.Target, params.Text)
		}
		if len(params.Wait.Until) != 2 || params.Wait.Until[0] != herdr.AgentStatusIdle || params.Wait.Until[1] != herdr.AgentStatusDone {
			t.Errorf("prompt wait until = %#v", params.Wait.Until)
		}
		if params.Wait.TimeoutMS == nil || *params.Wait.TimeoutMS != 1500 {
			t.Errorf("prompt timeout_ms = %v, want 1500", params.Wait.TimeoutMS)
		}
		return map[string]any{
			"id": requestID(t, request),
			"result": map[string]any{
				"type":  "agent_prompted",
				"agent": map[string]any{"terminal_id": "term_4", "agent": "codex", "agent_status": "idle", "workspace_id": "w2", "tab_id": "w2:t1", "pane_id": "w2:p4", "focused": false, "state_change_seq": 8, "revision": 2},
			},
		}
	})

	client, err := herdr.NewClient(socketPath)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	timeoutMS := uint64(1500)
	got, err := client.Prompt(context.Background(), "w2:p4", "review this", herdr.PromptOptions{
		Wait: &herdr.PromptWait{
			Until:     []herdr.AgentStatus{herdr.AgentStatusIdle, herdr.AgentStatusDone},
			TimeoutMS: &timeoutMS,
		},
	})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if got.PaneID != "w2:p4" {
		t.Fatalf("Prompt().PaneID = %q, want %q", got.PaneID, "w2:p4")
	}
}

func TestClientPromptOmitsWaitByDefault(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "herdr.sock")
	serveOne(t, socketPath, func(t *testing.T, request map[string]json.RawMessage) any {
		assertMethod(t, request, "agent.prompt")
		var params map[string]json.RawMessage
		decodeParams(t, request, &params)
		if _, present := params["wait"]; present {
			t.Errorf("prompt params contain wait: %s", params["wait"])
		}
		return map[string]any{
			"id": requestID(t, request),
			"result": map[string]any{
				"type":  "agent_prompted",
				"agent": map[string]any{"terminal_id": "term_4", "agent_status": "idle", "workspace_id": "w2", "tab_id": "w2:t1", "pane_id": "w2:p4", "focused": false, "revision": 2},
			},
		}
	})

	client, err := herdr.NewClient(socketPath)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if _, err := client.Prompt(context.Background(), "w2:p4", "continue", herdr.PromptOptions{}); err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
}

func TestClientPromptEncodesEmptyWaitForServerDefaults(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "herdr.sock")
	serveOne(t, socketPath, func(t *testing.T, request map[string]json.RawMessage) any {
		var params map[string]json.RawMessage
		decodeParams(t, request, &params)
		wait, present := params["wait"]
		if !present || string(wait) != "{}" {
			t.Errorf("prompt wait = %s, want empty object", wait)
		}
		return map[string]any{
			"id": requestID(t, request),
			"result": map[string]any{
				"type":  "agent_prompted",
				"agent": map[string]any{"terminal_id": "term_4", "agent_status": "idle", "workspace_id": "w2", "tab_id": "w2:t1", "pane_id": "w2:p4", "focused": false, "revision": 2},
			},
		}
	})

	client, err := herdr.NewClient(socketPath)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if _, err := client.Prompt(context.Background(), "w2:p4", "continue", herdr.PromptOptions{Wait: &herdr.PromptWait{}}); err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
}

func TestClientRejectsOversizedResponse(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "herdr.sock")
	serveOne(t, socketPath, func(t *testing.T, request map[string]json.RawMessage) any {
		return map[string]any{
			"id": requestID(t, request),
			"result": map[string]any{
				"type":     "pong",
				"version":  "0.7.5-with-padding-that-exceeds-the-test-limit",
				"protocol": 17,
			},
		}
	})

	client, err := herdr.NewClient(socketPath, herdr.WithMaxMessageBytes(64))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.Ping(context.Background())
	if !errors.Is(err, herdr.ErrMessageTooLarge) {
		t.Fatalf("Ping() error = %v, want ErrMessageTooLarge", err)
	}
}

func TestClientRejectsUnexpectedResultType(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "herdr.sock")
	serveOne(t, socketPath, func(t *testing.T, request map[string]json.RawMessage) any {
		return map[string]any{
			"id": requestID(t, request),
			"result": map[string]any{
				"type":     "agent_list",
				"version":  "0.7.5",
				"protocol": 17,
			},
		}
	})

	client, err := herdr.NewClient(socketPath)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.Ping(context.Background())
	var typeErr *herdr.UnexpectedResultError
	if !errors.As(err, &typeErr) {
		t.Fatalf("Ping() error = %v, want *UnexpectedResultError", err)
	}
	if typeErr.Want != "pong" || typeErr.Got != "agent_list" {
		t.Fatalf("UnexpectedResultError = %#v", typeErr)
	}
}

func TestClientRejectsMismatchedResponseID(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "herdr.sock")
	serveOne(t, socketPath, func(t *testing.T, request map[string]json.RawMessage) any {
		return map[string]any{
			"id":     "different-request",
			"result": map[string]any{"type": "pong", "version": "0.7.5", "protocol": 17},
		}
	})

	client, err := herdr.NewClient(socketPath)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.Ping(context.Background())
	var idErr *herdr.ResponseIDError
	if !errors.As(err, &idErr) {
		t.Fatalf("Ping() error = %v, want *ResponseIDError", err)
	}
	if idErr.ResponseID != "different-request" || idErr.RequestID == "" {
		t.Fatalf("ResponseIDError = %#v", idErr)
	}
}

func TestClientCancellationInterruptsResponseRead(t *testing.T) {
	t.Parallel()

	socketPath := filepath.Join(t.TempDir(), "herdr.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen on Unix socket: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	requestRead := make(chan struct{})
	serverDone := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverDone <- acceptErr
			return
		}
		defer connection.Close()
		reader := bufio.NewReader(connection)
		if _, readErr := reader.ReadBytes('\n'); readErr != nil {
			serverDone <- readErr
			return
		}
		close(requestRead)
		_, readErr := reader.ReadByte()
		serverDone <- readErr
	}()

	client, err := herdr.NewClient(socketPath)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, callErr := client.Ping(ctx)
		result <- callErr
	}()

	select {
	case <-requestRead:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("server did not receive request")
	}

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Ping() error = %v, want context.Canceled", err)
		}
		var transportErr *herdr.TransportError
		if !errors.As(err, &transportErr) || transportErr.Stage != herdr.TransportRead {
			t.Fatalf("Ping() error = %v, want read-stage TransportError", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Ping() did not stop after cancellation")
	}

	select {
	case <-serverDone:
	case <-time.After(time.Second):
		t.Fatal("server connection remained open after cancellation")
	}
}

func serveOne(t *testing.T, socketPath string, respond func(*testing.T, map[string]json.RawMessage) any) {
	t.Helper()

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen on Unix socket: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	done := make(chan struct{})
	go func() {
		defer close(done)

		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()

		line, readErr := bufio.NewReader(connection).ReadBytes('\n')
		if readErr != nil {
			t.Errorf("read request: %v", readErr)
			return
		}

		var request map[string]json.RawMessage
		if err := json.Unmarshal(line, &request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}

		if err := json.NewEncoder(connection).Encode(respond(t, request)); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}()

	t.Cleanup(func() {
		_ = listener.Close()
		<-done
	})
}

func assertMethod(t *testing.T, request map[string]json.RawMessage, want string) {
	t.Helper()

	var got string
	if err := json.Unmarshal(request["method"], &got); err != nil {
		t.Fatalf("decode method: %v", err)
	}
	if got != want {
		t.Errorf("method = %q, want %q", got, want)
	}
}

func decodeParams(t *testing.T, request map[string]json.RawMessage, destination any) {
	t.Helper()
	if err := json.Unmarshal(request["params"], destination); err != nil {
		t.Fatalf("decode params: %v", err)
	}
}

func assertEmptyParams(t *testing.T, request map[string]json.RawMessage) {
	t.Helper()

	var got map[string]any
	if err := json.Unmarshal(request["params"], &got); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("params = %#v, want empty object", got)
	}
}

func requestID(t *testing.T, request map[string]json.RawMessage) string {
	t.Helper()

	var id string
	if err := json.Unmarshal(request["id"], &id); err != nil {
		t.Fatalf("decode request id: %v", err)
	}
	if id == "" {
		t.Fatal("request id is empty")
	}
	return id
}
