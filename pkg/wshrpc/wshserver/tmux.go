// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package wshserver

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sanshao85/tideterm/pkg/genconn"
	"github.com/sanshao85/tideterm/pkg/remote"
	"github.com/sanshao85/tideterm/pkg/remote/conncontroller"
	"github.com/sanshao85/tideterm/pkg/util/shellutil"
	"github.com/sanshao85/tideterm/pkg/wshrpc"
	"github.com/sanshao85/tideterm/pkg/wslconn"
)

const tmuxSessionListFormat = "#{session_name}\t#{session_windows}\t#{session_attached}\t#{session_activity}\t#{@tideterm_alias}"
const tmuxSessionAliasOpt = "@tideterm_alias"

func (ws *WshServer) TmuxListSessionsCommand(ctx context.Context, data wshrpc.CommandTmuxListSessionsData) (*wshrpc.CommandTmuxListSessionsRtnData, error) {
	connName := strings.TrimSpace(data.ConnName)
	if connName == "" {
		return nil, fmt.Errorf("connection name is required")
	}
	if conncontroller.IsLocalConnName(connName) {
		return nil, fmt.Errorf("tmux session list is only available for remote connections")
	}

	ctx = genconn.ContextWithConnData(ctx, data.LogBlockId)
	ctx = termCtxWithLogBlockId(ctx, data.LogBlockId)

	shellClient, err := getRemoteShellClient(ctx, connName)
	if err != nil {
		return nil, err
	}

	stdout, stderr, err := runRemoteCommand(ctx, shellClient, fmt.Sprintf("tmux list-sessions -F '%s'", tmuxSessionListFormat))
	if err != nil {
		if isTmuxNoServer(stderr, err) {
			return &wshrpc.CommandTmuxListSessionsRtnData{Sessions: []wshrpc.TmuxSessionInfo{}}, nil
		}
		if isTmuxNotInstalled(stderr, err) {
			return nil, fmt.Errorf("tmux is not installed on %s", connName)
		}
		return nil, fmt.Errorf("tmux list-sessions failed: %w (stderr: %s)", err, strings.TrimSpace(stderr))
	}

	sessions := parseTmuxSessions(stdout)
	return &wshrpc.CommandTmuxListSessionsRtnData{Sessions: sessions}, nil
}

func (ws *WshServer) TmuxKillSessionCommand(ctx context.Context, data wshrpc.CommandTmuxKillSessionData) error {
	connName := strings.TrimSpace(data.ConnName)
	if connName == "" {
		return fmt.Errorf("connection name is required")
	}
	if conncontroller.IsLocalConnName(connName) {
		return fmt.Errorf("tmux session kill is only available for remote connections")
	}

	sessionName := strings.TrimSpace(data.SessionName)
	if sessionName == "" {
		return fmt.Errorf("session name is required")
	}

	ctx = genconn.ContextWithConnData(ctx, data.LogBlockId)
	ctx = termCtxWithLogBlockId(ctx, data.LogBlockId)

	shellClient, err := getRemoteShellClient(ctx, connName)
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf("tmux kill-session -t %s", shellutil.HardQuote(sessionName))
	_, stderr, err := runRemoteCommand(ctx, shellClient, cmd)
	if err != nil {
		if isTmuxNotInstalled(stderr, err) {
			return fmt.Errorf("tmux is not installed on %s", connName)
		}
		return fmt.Errorf("tmux kill-session failed: %w (stderr: %s)", err, strings.TrimSpace(stderr))
	}
	return nil
}

func (ws *WshServer) TmuxSetSessionAliasCommand(ctx context.Context, data wshrpc.CommandTmuxSetSessionAliasData) error {
	connName := strings.TrimSpace(data.ConnName)
	if connName == "" {
		return fmt.Errorf("connection name is required")
	}
	if conncontroller.IsLocalConnName(connName) {
		return fmt.Errorf("tmux session alias is only available for remote connections")
	}

	sessionName := strings.TrimSpace(data.SessionName)
	if sessionName == "" {
		return fmt.Errorf("session name is required")
	}

	alias := strings.TrimSpace(data.Alias)
	if strings.Contains(alias, "\n") || strings.Contains(alias, "\r") || strings.Contains(alias, "\t") {
		return fmt.Errorf("alias cannot contain newline or tab characters")
	}
	if len(alias) > 64 {
		return fmt.Errorf("alias is too long (max 64 characters)")
	}

	ctx = genconn.ContextWithConnData(ctx, data.LogBlockId)
	ctx = termCtxWithLogBlockId(ctx, data.LogBlockId)

	shellClient, err := getRemoteShellClient(ctx, connName)
	if err != nil {
		return err
	}

	quotedSession := shellutil.HardQuote(sessionName)
	if quotedSession == "" {
		return fmt.Errorf("invalid session name")
	}

	var cmd string
	if alias == "" {
		cmd = fmt.Sprintf("tmux set-option -u -t %s %s", quotedSession, tmuxSessionAliasOpt)
	} else {
		quotedAlias := shellutil.HardQuote(alias)
		if quotedAlias == "" {
			return fmt.Errorf("invalid alias")
		}
		cmd = fmt.Sprintf("tmux set-option -t %s %s %s", quotedSession, tmuxSessionAliasOpt, quotedAlias)
	}

	_, stderr, err := runRemoteCommand(ctx, shellClient, cmd)
	if err != nil {
		if isTmuxNotInstalled(stderr, err) {
			return fmt.Errorf("tmux is not installed on %s", connName)
		}
		return fmt.Errorf("tmux set-session-alias failed: %w (stderr: %s)", err, strings.TrimSpace(stderr))
	}

	return nil
}

func getRemoteShellClient(ctx context.Context, connName string) (genconn.ShellClient, error) {
	if strings.HasPrefix(connName, "wsl://") {
		distroName := strings.TrimPrefix(connName, "wsl://")
		if err := wslconn.EnsureConnection(ctx, distroName); err != nil {
			return nil, err
		}
		conn := wslconn.GetWslConn(distroName)
		if conn == nil {
			return nil, fmt.Errorf("connection not found: %s", connName)
		}
		client := conn.GetClient()
		if client == nil {
			return nil, fmt.Errorf("wsl client is not connected")
		}
		return genconn.MakeWSLShellClient(client), nil
	}

	if strings.HasPrefix(connName, "aws:") {
		return nil, fmt.Errorf("unsupported connection type: %s", connName)
	}

	if err := conncontroller.EnsureConnection(ctx, connName); err != nil {
		return nil, err
	}
	connOpts, err := remote.ParseOpts(connName)
	if err != nil {
		return nil, fmt.Errorf("error parsing connection name: %w", err)
	}
	conn := conncontroller.GetConn(connOpts)
	if conn == nil {
		return nil, fmt.Errorf("connection not found: %s", connName)
	}
	client := conn.GetClient()
	if client == nil {
		return nil, fmt.Errorf("ssh client is not connected")
	}
	return genconn.MakeSSHShellClient(client), nil
}

func runRemoteCommand(ctx context.Context, client genconn.ShellClient, cmd string) (string, string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	return genconn.RunSimpleCommand(cmdCtx, client, genconn.CommandSpec{Cmd: cmd})
}

func parseTmuxSessions(output string) []wshrpc.TmuxSessionInfo {
	output = strings.ReplaceAll(output, "\r\n", "\n")
	output = strings.ReplaceAll(output, "\r", "\n")
	lines := strings.Split(output, "\n")

	sessions := make([]wshrpc.TmuxSessionInfo, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) < 4 {
			continue
		}
		name := parts[0]
		alias := ""
		if len(parts) > 4 {
			alias = strings.TrimSpace(parts[4])
		}
		windows, _ := strconv.Atoi(parts[1])
		attached, _ := strconv.Atoi(parts[2])
		activity, _ := strconv.ParseInt(parts[3], 10, 64)
		sessions = append(sessions, wshrpc.TmuxSessionInfo{
			Name:     name,
			Alias:    alias,
			Windows:  windows,
			Attached: attached,
			Activity: activity,
		})
	}
	return sessions
}

func isTmuxNoServer(stderr string, err error) bool {
	text := strings.ToLower(strings.TrimSpace(stderr))
	if text == "" && err != nil {
		text = strings.ToLower(err.Error())
	}
	return strings.Contains(text, "no server running") || strings.Contains(text, "failed to connect to server")
}

func isTmuxNotInstalled(stderr string, err error) bool {
	text := strings.ToLower(strings.TrimSpace(stderr))
	if text == "" && err != nil {
		text = strings.ToLower(err.Error())
	}
	return strings.Contains(text, "command not found") || strings.Contains(text, "tmux: not found")
}
