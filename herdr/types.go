package herdr

// AgentStatus is Herdr's semantic agent state.
type AgentStatus string

const (
	AgentStatusIdle    AgentStatus = "idle"
	AgentStatusWorking AgentStatus = "working"
	AgentStatusBlocked AgentStatus = "blocked"
	AgentStatusDone    AgentStatus = "done"
	AgentStatusUnknown AgentStatus = "unknown"
)

// PromptOptions controls optional server-side waiting after prompt submission.
type PromptOptions struct {
	Wait *PromptWait
}

// PromptWait asks Herdr to wait for one of Until after submitting a prompt.
type PromptWait struct {
	Until     []AgentStatus `json:"until,omitempty"`
	TimeoutMS *uint64       `json:"timeout_ms,omitempty"`
}

// AgentSessionKind identifies how an agent session can be resumed.
type AgentSessionKind string

const (
	AgentSessionID   AgentSessionKind = "id"
	AgentSessionPath AgentSessionKind = "path"
)

// AgentSession identifies a stable native agent session reported by Herdr.
type AgentSession struct {
	Source string           `json:"source"`
	Agent  string           `json:"agent"`
	Kind   AgentSessionKind `json:"kind"`
	Value  string           `json:"value"`
}

// AgentInfo describes one detected agent occupant.
type AgentInfo struct {
	TerminalID             string            `json:"terminal_id"`
	Name                   *string           `json:"name,omitempty"`
	Agent                  *string           `json:"agent,omitempty"`
	Title                  *string           `json:"title,omitempty"`
	TerminalTitle          *string           `json:"terminal_title,omitempty"`
	TerminalTitleStripped  *string           `json:"terminal_title_stripped,omitempty"`
	DisplayAgent           *string           `json:"display_agent,omitempty"`
	Status                 AgentStatus       `json:"agent_status"`
	ScreenDetectionSkipped bool              `json:"screen_detection_skipped,omitempty"`
	StateLabels            map[string]string `json:"state_labels,omitempty"`
	Tokens                 map[string]string `json:"tokens,omitempty"`
	AgentSession           *AgentSession     `json:"agent_session,omitempty"`
	WorkspaceID            string            `json:"workspace_id"`
	TabID                  string            `json:"tab_id"`
	PaneID                 string            `json:"pane_id"`
	Focused                bool              `json:"focused"`
	LaunchPending          bool              `json:"launch_pending,omitempty"`
	InteractiveReady       bool              `json:"interactive_ready,omitempty"`
	StateChangeSeq         uint64            `json:"state_change_seq"`
	CWD                    *string           `json:"cwd,omitempty"`
	ForegroundCWD          *string           `json:"foreground_cwd,omitempty"`
	Revision               uint64            `json:"revision"`
}

// WorkspaceInfo describes a Herdr workspace in a snapshot.
type WorkspaceInfo struct {
	WorkspaceID string                 `json:"workspace_id"`
	Number      int                    `json:"number"`
	Label       string                 `json:"label"`
	Focused     bool                   `json:"focused"`
	PaneCount   int                    `json:"pane_count"`
	TabCount    int                    `json:"tab_count"`
	ActiveTabID string                 `json:"active_tab_id"`
	AgentStatus AgentStatus            `json:"agent_status"`
	Tokens      map[string]string      `json:"tokens,omitempty"`
	Worktree    *WorkspaceWorktreeInfo `json:"worktree,omitempty"`
}

// WorkspaceWorktreeInfo identifies the checkout associated with a workspace.
type WorkspaceWorktreeInfo struct {
	RepoKey          string `json:"repo_key"`
	RepoName         string `json:"repo_name"`
	RepoRoot         string `json:"repo_root"`
	CheckoutPath     string `json:"checkout_path"`
	IsLinkedWorktree bool   `json:"is_linked_worktree"`
}

// TabInfo describes a Herdr tab in a snapshot.
type TabInfo struct {
	TabID       string      `json:"tab_id"`
	WorkspaceID string      `json:"workspace_id"`
	Number      int         `json:"number"`
	Label       string      `json:"label"`
	Focused     bool        `json:"focused"`
	PaneCount   int         `json:"pane_count"`
	AgentStatus AgentStatus `json:"agent_status"`
}

// PaneInfo describes a Herdr pane in a snapshot.
type PaneInfo struct {
	PaneID                string            `json:"pane_id"`
	TerminalID            string            `json:"terminal_id"`
	WorkspaceID           string            `json:"workspace_id"`
	TabID                 string            `json:"tab_id"`
	Focused               bool              `json:"focused"`
	CWD                   *string           `json:"cwd,omitempty"`
	ForegroundCWD         *string           `json:"foreground_cwd,omitempty"`
	Label                 *string           `json:"label,omitempty"`
	Agent                 *string           `json:"agent,omitempty"`
	Title                 *string           `json:"title,omitempty"`
	TerminalTitle         *string           `json:"terminal_title,omitempty"`
	TerminalTitleStripped *string           `json:"terminal_title_stripped,omitempty"`
	DisplayAgent          *string           `json:"display_agent,omitempty"`
	AgentStatus           AgentStatus       `json:"agent_status"`
	StateLabels           map[string]string `json:"state_labels,omitempty"`
	Tokens                map[string]string `json:"tokens,omitempty"`
	AgentSession          *AgentSession     `json:"agent_session,omitempty"`
	Scroll                *PaneScrollInfo   `json:"scroll,omitempty"`
	Revision              uint64            `json:"revision"`
}

// PaneScrollInfo reports terminal scroll position.
type PaneScrollInfo struct {
	OffsetFromBottom    uint64 `json:"offset_from_bottom"`
	MaxOffsetFromBottom uint64 `json:"max_offset_from_bottom"`
	ViewportRows        uint64 `json:"viewport_rows"`
}

// PaneLayoutSnapshot describes the visible split geometry of a tab.
type PaneLayoutSnapshot struct {
	WorkspaceID   string            `json:"workspace_id"`
	TabID         string            `json:"tab_id"`
	Zoomed        bool              `json:"zoomed"`
	Area          PaneLayoutRect    `json:"area"`
	FocusedPaneID string            `json:"focused_pane_id"`
	Panes         []PaneLayoutPane  `json:"panes"`
	Splits        []PaneLayoutSplit `json:"splits"`
}

// PaneLayoutRect is a terminal-cell rectangle.
type PaneLayoutRect struct {
	X      uint16 `json:"x"`
	Y      uint16 `json:"y"`
	Width  uint16 `json:"width"`
	Height uint16 `json:"height"`
}

// PaneLayoutPane places a pane in a layout.
type PaneLayoutPane struct {
	PaneID  string         `json:"pane_id"`
	Focused bool           `json:"focused"`
	Rect    PaneLayoutRect `json:"rect"`
}

// PaneLayoutSplit describes one split in a layout.
type PaneLayoutSplit struct {
	ID        string         `json:"id"`
	Direction string         `json:"direction"`
	Ratio     float32        `json:"ratio"`
	Rect      PaneLayoutRect `json:"rect"`
}

// SessionSnapshot is the authoritative one-time session state returned by Herdr.
type SessionSnapshot struct {
	Version            string               `json:"version"`
	Protocol           uint32               `json:"protocol"`
	FocusedWorkspaceID *string              `json:"focused_workspace_id,omitempty"`
	FocusedTabID       *string              `json:"focused_tab_id,omitempty"`
	FocusedPaneID      *string              `json:"focused_pane_id,omitempty"`
	Workspaces         []WorkspaceInfo      `json:"workspaces"`
	Tabs               []TabInfo            `json:"tabs"`
	Panes              []PaneInfo           `json:"panes"`
	Layouts            []PaneLayoutSnapshot `json:"layouts"`
	Agents             []AgentInfo          `json:"agents"`
}
