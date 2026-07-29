// Package herdr implements a typed client for the Herdr socket API.
package herdr

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strings"
	"sync/atomic"
)

const defaultMaxMessageBytes int64 = 8 << 20

// ProtocolVersion is the oldest Herdr protocol baseline modeled by this package.
// Optional capability-gated fields may be available on newer servers.
const ProtocolVersion uint32 = 17

// ErrMessageTooLarge reports an NDJSON response above the configured limit.
var ErrMessageTooLarge = errors.New("herdr: response exceeds maximum message size")

// Client sends requests to one Herdr Unix socket.
type Client struct {
	socketPath      string
	maxMessageBytes int64
	nextID          atomic.Uint64
}

// Option configures a Client.
type Option func(*Client) error

// WithMaxMessageBytes limits one NDJSON response. The default is 8 MiB.
func WithMaxMessageBytes(limit int64) Option {
	return func(client *Client) error {
		if limit <= 0 || limit == math.MaxInt64 {
			return errors.New("herdr: maximum message size must be positive and below MaxInt64")
		}
		client.maxMessageBytes = limit
		return nil
	}
}

// NewClient creates a client for the exact socketPath. It does not discover or
// authenticate the endpoint; callers that cross a trust boundary must validate
// the path and peer before use.
func NewClient(socketPath string, options ...Option) (*Client, error) {
	if strings.TrimSpace(socketPath) == "" {
		return nil, errors.New("herdr: socket path is empty")
	}
	client := &Client{socketPath: socketPath, maxMessageBytes: defaultMaxMessageBytes}
	for _, option := range options {
		if option == nil {
			return nil, errors.New("herdr: nil client option")
		}
		if err := option(client); err != nil {
			return nil, err
		}
	}
	return client, nil
}

// ServerCapabilities describes optional server features advertised by ping.
type ServerCapabilities struct {
	LiveHandoff                bool `json:"live_handoff"`
	DetachedServerDaemon       bool `json:"detached_server_daemon"`
	AgentPromptExpectedSession bool `json:"agent_prompt_expected_session"`
}

// ServerInfo is returned by Ping.
type ServerInfo struct {
	Type         string              `json:"type"`
	Version      string              `json:"version"`
	Protocol     uint32              `json:"protocol"`
	Capabilities *ServerCapabilities `json:"capabilities,omitempty"`
}

// Ping reports the server version, protocol, and capabilities.
func (c *Client) Ping(ctx context.Context) (ServerInfo, error) {
	var result ServerInfo
	if err := c.call(ctx, "ping", struct{}{}, "pong", &result); err != nil {
		return ServerInfo{}, err
	}
	return result, nil
}

// Snapshot returns Herdr's authoritative one-time session snapshot.
func (c *Client) Snapshot(ctx context.Context) (SessionSnapshot, error) {
	var result struct {
		Snapshot SessionSnapshot `json:"snapshot"`
	}
	if err := c.call(ctx, "session.snapshot", struct{}{}, "session_snapshot", &result); err != nil {
		return SessionSnapshot{}, err
	}
	return result.Snapshot, nil
}

// ListAgents returns all detected agent occupants.
func (c *Client) ListAgents(ctx context.Context) ([]AgentInfo, error) {
	var result struct {
		Agents []AgentInfo `json:"agents"`
	}
	if err := c.call(ctx, "agent.list", struct{}{}, "agent_list", &result); err != nil {
		return nil, err
	}
	return result.Agents, nil
}

// GetAgent resolves target as a Herdr agent name or pane ID.
func (c *Client) GetAgent(ctx context.Context, target string) (AgentInfo, error) {
	var result struct {
		Agent AgentInfo `json:"agent"`
	}
	if err := c.call(ctx, "agent.get", struct {
		Target string `json:"target"`
	}{Target: target}, "agent_info", &result); err != nil {
		return AgentInfo{}, err
	}
	return result.Agent, nil
}

// Prompt submits text to a Herdr agent target and optionally waits for lifecycle
// state. The wait is not turn correlation. ExpectedSession is safe only after an
// affirmative AgentPromptExpectedSession capability check; older servers may
// ignore the field. Except for a received agent_session_mismatch rejection, any
// non-dial failure is returned as an AmbiguousPromptError and must not be retried
// automatically.
func (c *Client) Prompt(ctx context.Context, target, text string, options PromptOptions) (AgentInfo, error) {
	params := struct {
		Target          string        `json:"target"`
		Text            string        `json:"text"`
		ExpectedSession *AgentSession `json:"expected_session,omitempty"`
		Wait            *PromptWait   `json:"wait,omitempty"`
	}{
		Target:          target,
		Text:            text,
		ExpectedSession: options.ExpectedSession,
		Wait:            options.Wait,
	}
	var result struct {
		Agent AgentInfo `json:"agent"`
	}
	if err := c.call(ctx, "agent.prompt", params, "agent_prompted", &result); err != nil {
		var transportErr *TransportError
		if errors.As(err, &transportErr) && transportErr.Stage == TransportDial {
			return AgentInfo{}, err
		}
		var apiErr *APIError
		if options.ExpectedSession != nil && errors.As(err, &apiErr) && apiErr.Code == ErrorCodeAgentSessionMismatch {
			return AgentInfo{}, err
		}
		return AgentInfo{}, &AmbiguousPromptError{Err: err}
	}
	return result.Agent, nil
}

type request struct {
	ID     string `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params"`
}

type response struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *APIError       `json:"error"`
}

// ErrorCodeAgentSessionMismatch is a known rejection before prompt submission.
const ErrorCodeAgentSessionMismatch = "agent_session_mismatch"

// APIError is an error returned by the Herdr server.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error implements error.
func (e *APIError) Error() string {
	return fmt.Sprintf("herdr: API error %s: %s", e.Code, e.Message)
}

// InvalidResponseError reports an invalid response envelope.
type InvalidResponseError struct {
	Reason string
}

// Error implements error.
func (e *InvalidResponseError) Error() string {
	return "herdr: invalid response: " + e.Reason
}

// AmbiguousPromptError reports that a prompt might have been accepted even
// though the client could not establish its outcome. Callers must not retry it
// automatically. The wrapped error remains available through errors.Is/As.
type AmbiguousPromptError struct {
	Err error
}

// Error implements error.
func (e *AmbiguousPromptError) Error() string {
	return "herdr: prompt outcome is ambiguous: " + e.Err.Error()
}

// Unwrap returns the underlying API, protocol, or transport error.
func (e *AmbiguousPromptError) Unwrap() error {
	return e.Err
}

// UnexpectedResultError reports a success response for a different method shape.
type UnexpectedResultError struct {
	Method string
	Want   string
	Got    string
}

// Error implements error.
func (e *UnexpectedResultError) Error() string {
	return fmt.Sprintf("herdr: %s returned result type %q, want %q", e.Method, e.Got, e.Want)
}

// ResponseIDError reports a response correlated to another request.
type ResponseIDError struct {
	RequestID  string
	ResponseID string
}

// Error implements error.
func (e *ResponseIDError) Error() string {
	return fmt.Sprintf("herdr: response id %q does not match request id %q", e.ResponseID, e.RequestID)
}

// TransportStage identifies the socket operation that failed.
type TransportStage string

const (
	TransportDial  TransportStage = "dial"
	TransportWrite TransportStage = "write"
	TransportRead  TransportStage = "read"
)

// TransportError reports a socket failure and preserves its underlying error.
// A write- or read-stage error after Prompt may mean the server accepted the mutation.
type TransportError struct {
	Stage TransportStage
	Err   error
}

// Error implements error.
func (e *TransportError) Error() string {
	return fmt.Sprintf("herdr: %s: %v", e.Stage, e.Err)
}

// Unwrap returns the underlying socket or context error.
func (e *TransportError) Unwrap() error {
	return e.Err
}

func (c *Client) call(ctx context.Context, method string, params any, wantType string, result any) error {
	connection, err := (&net.Dialer{}).DialContext(ctx, "unix", c.socketPath)
	if err != nil {
		return &TransportError{Stage: TransportDial, Err: err}
	}
	defer connection.Close()

	stopCancel := context.AfterFunc(ctx, func() { _ = connection.Close() })
	defer stopCancel()

	id := fmt.Sprintf("herdr-go-%d", c.nextID.Add(1))
	if err := json.NewEncoder(connection).Encode(request{ID: id, Method: method, Params: params}); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return &TransportError{Stage: TransportWrite, Err: ctxErr}
		}
		return &TransportError{Stage: TransportWrite, Err: err}
	}

	limited := io.LimitReader(connection, c.maxMessageBytes+1)
	line, err := bufio.NewReader(limited).ReadBytes('\n')
	if int64(len(line)) > c.maxMessageBytes {
		return ErrMessageTooLarge
	}
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return &TransportError{Stage: TransportRead, Err: ctxErr}
		}
		return &TransportError{Stage: TransportRead, Err: err}
	}

	var envelope response
	if err := json.Unmarshal(line, &envelope); err != nil {
		return fmt.Errorf("herdr: decode response: %w", err)
	}
	if envelope.ID != id {
		return &ResponseIDError{RequestID: id, ResponseID: envelope.ID}
	}
	trimmedResult := bytes.TrimSpace(envelope.Result)
	hasResult := len(trimmedResult) != 0 && !bytes.Equal(trimmedResult, []byte("null"))
	if envelope.Error != nil && hasResult {
		return &InvalidResponseError{Reason: "contains both result and error"}
	}
	if envelope.Error != nil {
		return envelope.Error
	}
	if !hasResult {
		return &InvalidResponseError{Reason: "contains neither result nor error"}
	}
	var header struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(envelope.Result, &header); err != nil {
		return fmt.Errorf("herdr: decode result type: %w", err)
	}
	if header.Type != wantType {
		return &UnexpectedResultError{Method: method, Want: wantType, Got: header.Type}
	}
	if err := json.Unmarshal(envelope.Result, result); err != nil {
		return fmt.Errorf("herdr: decode result: %w", err)
	}
	return nil
}
