package extensions

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stelmakhdigital/stell-coding/internal/version"
)

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
}

type SlashCommandDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CommandResult struct {
	Message string `json:"message,omitempty"`
}

type ReloadStatus struct {
	Package   string `json:"package"`
	Extension string `json:"extension"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ProcessClient struct {
	mu      sync.Mutex
	enc     *json.Encoder
	pending map[int]chan rpcResponse
	nextID  atomic.Int32
	cmd     *exec.Cmd
	done    chan struct{}
	Notify  func(method string, params map[string]any)
	HostRequest func(method string, params json.RawMessage) (any, error)
}

func (c *ProcessClient) Exited() <-chan struct{} { return c.done }

func StartProcess(ctx context.Context, dir string, command []string) (*ProcessClient, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("empty extension command")
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = dir
	configureProcessGroup(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c := &ProcessClient{
		enc:     json.NewEncoder(stdin),
		pending: map[int]chan rpcResponse{},
		cmd:     cmd,
		done:    make(chan struct{}),
	}
	go c.readLoop(stdout)
	go func() {
		_ = cmd.Wait()
		close(c.done)
	}()
	return c, nil
}

func (c *ProcessClient) readLoop(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		var head struct {
			ID      int             `json:"id"`
			Method  string          `json:"method"`
			Result  json.RawMessage `json:"result"`
			Error   *rpcError       `json:"error"`
			Params  json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(line, &head); err != nil {
			continue
		}
		c.mu.Lock()
		ch := c.pending[head.ID]
		if ch != nil && head.Method == "" {
			delete(c.pending, head.ID)
		}
		notify := c.Notify
		hostFn := c.HostRequest
		enc := c.enc
		c.mu.Unlock()

		if head.Method != "" && head.ID != 0 {
			req := rpcRequest{JSONRPC: "2.0", ID: head.ID, Method: head.Method, Params: head.Params}
			c.handleHostInbound(hostFn, enc, req)
			continue
		}
		if ch != nil && head.Method == "" {
			ch <- rpcResponse{JSONRPC: "2.0", ID: head.ID, Result: head.Result, Error: head.Error}
			continue
		}
		if notify == nil || head.Method == "" {
			continue
		}
		var params map[string]any
		if len(head.Params) > 0 {
			_ = json.Unmarshal(head.Params, &params)
		}
		notify(head.Method, params)
	}
	_ = sc.Err()
}

func (c *ProcessClient) handleHostInbound(hostFn func(method string, params json.RawMessage) (any, error), enc *json.Encoder, req rpcRequest) {
	if hostFn == nil || enc == nil {
		return
	}
	result, err := hostFn(req.Method, req.Params)
	out := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	if err != nil {
		out.Error = &rpcError{Code: -32000, Message: err.Error()}
	} else if result != nil {
		raw, _ := json.Marshal(result)
		out.Result = raw
	} else {
		out.Result = json.RawMessage(`{}`)
	}
	c.mu.Lock()
	_ = enc.Encode(out)
	c.mu.Unlock()
}

func (c *ProcessClient) SendNotify(method string, params any) error {
	if c == nil {
		return fmt.Errorf("nil client")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.enc == nil {
		return fmt.Errorf("client closed")
	}
	return c.enc.Encode(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
}

func (c *ProcessClient) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := int(c.nextID.Add(1))
	var rawParams json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		rawParams = b
	}
	req := rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: rawParams}
	ch := make(chan rpcResponse, 1)
	c.mu.Lock()
	c.pending[id] = ch
	if err := c.enc.Encode(req); err != nil {
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}
	c.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, fmt.Errorf("extension process exited")
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("extension error: %s", resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (c *ProcessClient) Close() error {
	terminateProcessTree(c.cmd)
	select {
	case <-c.done:
	case <-time.After(2 * time.Second):
		if c.cmd != nil && c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
	}
	return nil
}

type InitResult struct {
	Tools    []ToolDef
	Commands []SlashCommandDef
}

func (c *ProcessClient) Initialize(ctx context.Context, workspace, pkgName string) (InitResult, error) {
	raw, err := c.Call(ctx, "initialize", map[string]any{
		"stellVersion": version.Version,
		"workspace":    workspace,
		"package":      pkgName,
	})
	if err != nil {
		return InitResult{}, err
	}
	var res struct {
		Tools    []ToolDef         `json:"tools"`
		Commands []SlashCommandDef `json:"commands"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &res); err != nil {
			return InitResult{}, err
		}
	}
	return InitResult{Tools: res.Tools, Commands: res.Commands}, nil
}

func (c *ProcessClient) InvokeTool(ctx context.Context, name string, args map[string]any) (string, error) {
	raw, err := c.Call(ctx, "tools/invoke", map[string]any{"name": name, "args": args})
	if err != nil {
		return "", err
	}
	var res struct {
		Content string `json:"content"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return string(raw), nil
	}
	if res.Error != "" {
		return "", fmt.Errorf("%s", res.Error)
	}
	return res.Content, nil
}

func (c *ProcessClient) EmitHook(ctx context.Context, name string, payload map[string]any) (map[string]any, error) {
	raw, err := c.Call(ctx, "hook/"+name, payload)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var res map[string]any
	if err := json.Unmarshal(raw, &res); err != nil {
		return map[string]any{"raw": string(raw)}, nil
	}
	return res, nil
}

func (c *ProcessClient) InvokeCommand(ctx context.Context, name string, args []string, sessionID string) (CommandResult, error) {
	raw, err := c.Call(ctx, "commands/invoke", map[string]any{
		"name": name, "args": args, "sessionId": sessionID,
	})
	if err != nil {
		return CommandResult{}, err
	}
	var res CommandResult
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &res)
	}
	return res, nil
}

func (c *ProcessClient) InvokeShortcut(ctx context.Context, action string) error {
	_, err := c.Call(ctx, "shortcuts/invoke", map[string]any{"action": action})
	return err
}

func (c *ProcessClient) CancelWorkflow(ctx context.Context, runID string) error {
	_, err := c.Call(ctx, "workflow/cancel", map[string]any{"runId": runID})
	return err
}
