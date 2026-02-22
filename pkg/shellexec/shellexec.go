// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package shellexec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"maps"

	"github.com/creack/pty"
	"github.com/sanshao85/tideterm/pkg/blocklogger"
	"github.com/sanshao85/tideterm/pkg/panichandler"
	"github.com/sanshao85/tideterm/pkg/remote/conncontroller"
	"github.com/sanshao85/tideterm/pkg/util/pamparse"
	"github.com/sanshao85/tideterm/pkg/util/shellutil"
	"github.com/sanshao85/tideterm/pkg/util/utilfn"
	"github.com/sanshao85/tideterm/pkg/wavebase"
	"github.com/sanshao85/tideterm/pkg/waveobj"
	"github.com/sanshao85/tideterm/pkg/wconfig"
	"github.com/sanshao85/tideterm/pkg/wshrpc"
	"github.com/sanshao85/tideterm/pkg/wshrpc/wshclient"
	"github.com/sanshao85/tideterm/pkg/wshutil"
	"github.com/sanshao85/tideterm/pkg/wslconn"
)

const DefaultGracefulKillWait = 400 * time.Millisecond

type CommandOptsType struct {
	Interactive bool                      `json:"interactive,omitempty"`
	Login       bool                      `json:"login,omitempty"`
	Cwd         string                    `json:"cwd,omitempty"`
	ShellPath   string                    `json:"shellPath,omitempty"`
	ShellOpts   []string                  `json:"shellOpts,omitempty"`
	SwapToken   *shellutil.TokenSwapEntry `json:"swapToken,omitempty"`
	ForceJwt    bool                      `json:"forcejwt,omitempty"`
}

type ShellProc struct {
	ConnName  string
	Cmd       ConnInterface
	CloseOnce *sync.Once
	DoneCh    chan any // closed after proc.Wait() returns
	WaitErr   error    // WaitErr is synchronized by DoneCh (written before DoneCh is closed) and CloseOnce
}

func (sp *ShellProc) Close() {
	sp.Cmd.KillGraceful(DefaultGracefulKillWait)
	go func() {
		defer func() {
			panichandler.PanicHandler("ShellProc.Close", recover())
		}()
		waitErr := sp.Cmd.Wait()
		sp.SetWaitErrorAndSignalDone(waitErr)

		// windows cannot handle the pty being
		// closed twice, so we let the pty
		// close itself instead
		if runtime.GOOS != "windows" {
			sp.Cmd.Close()
		}
	}()
}

func (sp *ShellProc) SetWaitErrorAndSignalDone(waitErr error) {
	sp.CloseOnce.Do(func() {
		sp.WaitErr = waitErr
		close(sp.DoneCh)
	})
}

func (sp *ShellProc) Wait() error {
	<-sp.DoneCh
	return sp.WaitErr
}

// returns (done, waitError)
func (sp *ShellProc) WaitNB() (bool, error) {
	select {
	case <-sp.DoneCh:
		return true, sp.WaitErr
	default:
		return false, nil
	}
}

func ExitCodeFromWaitErr(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}
	return -1

}

func checkCwd(cwd string) error {
	if cwd == "" {
		return fmt.Errorf("cwd is empty")
	}
	if _, err := os.Stat(cwd); err != nil {
		return fmt.Errorf("error statting cwd %q: %w", cwd, err)
	}
	return nil
}

func escapeForPosixDoubleQuotes(s string) string {
	// Conservative escaping for the subset of chars that are special inside double quotes.
	// This is used for "$HOME<rest>" where <rest> should be treated literally.
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '"', '$', '`':
			b.WriteByte('\\')
			b.WriteByte(s[i])
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func posixCwdExpr(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return ""
	}
	if cwd == "~" {
		return "~"
	}
	if strings.HasPrefix(cwd, "~/") {
		// "~" must be expanded on the target machine. Use $HOME so we can still quote paths with spaces safely.
		rest := cwd[1:] // includes leading "/"
		return fmt.Sprintf("\"$HOME%s\"", escapeForPosixDoubleQuotes(rest))
	}
	return utilfn.ShellQuote(cwd, false, -1)
}

func posixCwdExprNoWshRemote(cwd string, sshUser string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return ""
	}
	sshUser = strings.TrimSpace(sshUser)
	if sshUser == "" {
		return posixCwdExpr(cwd)
	}
	if cwd == "~" {
		// Prefer ~user so we don't depend on $HOME being correct on the remote shell.
		return "~" + sshUser
	}
	if cwd == "~/" {
		return "~" + sshUser + "/"
	}
	if strings.HasPrefix(cwd, "~/") {
		// Prefer ~user so we don't depend on $HOME being correct on the remote shell.
		rest := cwd[1:] // includes leading "/"
		return "~" + sshUser + rest
	}
	return posixCwdExpr(cwd)
}

func fishCwdExpr(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return ""
	}
	if cwd == "~" {
		return "~"
	}
	if strings.HasPrefix(cwd, "~/") {
		// "~" must be expanded on the target machine. Use $HOME so we can still quote paths with spaces safely.
		rest := cwd[1:] // includes leading "/"
		return fmt.Sprintf("\"$HOME%s\"", escapeForPosixDoubleQuotes(rest))
	}
	return shellutil.HardQuoteFish(cwd)
}

func makeCwdInitScript(shellType string, cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return ""
	}
	switch shellType {
	case shellutil.ShellType_pwsh:
		quoted := shellutil.HardQuotePowerShell(cwd)
		if quoted == "" {
			return ""
		}
		return fmt.Sprintf("try { Set-Location -Path %s } catch {}\n", quoted)
	case shellutil.ShellType_fish:
		cwdExpr := fishCwdExpr(cwd)
		if cwdExpr == "" {
			return ""
		}
		// Fish doesn't use "||"; a failed cd won't abort the shell, so keep it simple.
		return fmt.Sprintf("cd %s\n", cwdExpr)
	default:
		cwdExpr := posixCwdExpr(cwd)
		if cwdExpr == "" {
			return ""
		}
		return fmt.Sprintf("cd -- %s || true\n", cwdExpr)
	}
}

func prependCwdInitScriptToSwapToken(shellType string, cwd string, swapToken *shellutil.TokenSwapEntry) {
	if swapToken == nil {
		return
	}
	initScript := makeCwdInitScript(shellType, cwd)
	if initScript == "" {
		return
	}
	swapToken.ScriptText = initScript + swapToken.ScriptText
}

func makeHomeResetInitScript(shellType string) string {
	switch shellType {
	case shellutil.ShellType_bash, shellutil.ShellType_zsh:
		// Some environments override HOME unexpectedly (e.g. to /tmp), which breaks "~" expansion
		// and any "$HOME/..." paths injected by TideTerm (e.g. remote cwd init scripts).
		// Prefer the passwd entry for the current user.
		return "if command -v id >/dev/null 2>&1; then\n" +
			"  _waveterm_user=\"$(id -un 2>/dev/null)\" || _waveterm_user=\"\"\n" +
			"  if [ -n \"$_waveterm_user\" ]; then\n" +
			"    _waveterm_home=\"$(eval echo ~${_waveterm_user} 2>/dev/null)\" || _waveterm_home=\"\"\n" +
			"    if [ -n \"$_waveterm_home\" ] && [ \"$_waveterm_home\" != \"$HOME\" ]; then\n" +
			"      export HOME=\"$_waveterm_home\"\n" +
			"    fi\n" +
			"  fi\n" +
			"  unset _waveterm_user _waveterm_home\n" +
			"fi\n"
	default:
		return ""
	}
}

func prependHomeResetInitScriptToSwapToken(shellType string, swapToken *shellutil.TokenSwapEntry) {
	if swapToken == nil {
		return
	}
	initScript := makeHomeResetInitScript(shellType)
	if initScript == "" {
		return
	}
	swapToken.ScriptText = initScript + swapToken.ScriptText
}

func isChineseAppLanguage() bool {
	lang := strings.TrimSpace(wconfig.GetWatcher().GetFullConfig().Settings.AppLanguage)
	if lang == "" {
		return false
	}
	return strings.HasPrefix(strings.ToLower(lang), "zh")
}

func tmuxSessionNameForBlockId(blockId string) string {
	blockId = strings.TrimSpace(blockId)
	if blockId == "" {
		return ""
	}
	// Keep only safe chars for tmux session names.
	var b strings.Builder
	b.Grow(len(blockId))
	for _, r := range blockId {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			// drop "-" to keep names short; "_" is kept as-is
			if r == '_' {
				b.WriteRune(r)
			}
		}
	}
	cleaned := b.String()
	if cleaned == "" {
		return ""
	}
	return "tideterm-" + cleaned
}

func tmuxNotInstalledMessageLines(zh bool) []string {
	if zh {
		return []string{
			"提示：此主机未安装 tmux，因此无法启用 TideTerm 远程终端“断线续连”。",
			"安装 tmux 后，电脑休眠/网络中断导致断线时，重连可继续之前的终端会话（适合 codex/claude 等交互程序）。",
			"安装示例：Ubuntu/Debian: sudo apt-get install -y tmux",
			"安装示例：CentOS/RHEL: sudo yum install -y tmux（或 sudo dnf install -y tmux）",
			"安装后重开该终端块即可生效。",
		}
	}
	return []string{
		"Tip: tmux is not installed on this host, so TideTerm cannot enable remote terminal resume.",
		"Install tmux to keep interactive sessions running across reconnects (sleep/network drop), e.g. codex/claude.",
		"Install (Ubuntu/Debian): sudo apt-get install -y tmux",
		"Install (CentOS/RHEL): sudo yum install -y tmux (or sudo dnf install -y tmux)",
		"After installing, reopen this terminal block.",
	}
}

func makeTmuxAutoResumeShellCommand(sessionName string, zh bool) string {
	// Use sh so this works even if the user's login shell is fish/zsh/etc.
	msgLines := tmuxNotInstalledMessageLines(zh)
	var printfArgs strings.Builder
	for _, line := range msgLines {
		printfArgs.WriteString(" \"")
		printfArgs.WriteString(escapeForPosixDoubleQuotes(line))
		printfArgs.WriteString("\"")
	}
	return fmt.Sprintf(
		"sh -lc 'if [ -n \"${TMUX-}\" ] || [ -n \"${STY-}\" ]; then exit 0; fi; "+
			// Ensure HOME matches the passwd entry (some environments override HOME to /tmp, etc.).
			"if command -v id >/dev/null 2>&1; then "+
			"  _tideterm_user=\"$(id -un 2>/dev/null)\" || _tideterm_user=\"\"; "+
			"  if [ -n \"$_tideterm_user\" ]; then "+
			"    _tideterm_home=\"$(eval echo ~${_tideterm_user} 2>/dev/null)\" || _tideterm_home=\"\"; "+
			"    if [ -n \"$_tideterm_home\" ] && [ \"$_tideterm_home\" != \"$HOME\" ]; then export HOME=\"$_tideterm_home\"; fi; "+
			"  fi; "+
			"  unset _tideterm_user _tideterm_home; "+
			"fi; "+
			"if command -v tmux >/dev/null 2>&1; then "+
			"  unset TIDETERM_SWAPTOKEN 2>/dev/null || true; "+
			"  _tideterm_wsh_path=\"$(command -v wsh 2>/dev/null)\" || _tideterm_wsh_path=\"\"; "+
			"  _tideterm_wsh_dir=\"\"; "+
			"  if [ -n \"$_tideterm_wsh_path\" ]; then _tideterm_wsh_dir=\"$(dirname \"$_tideterm_wsh_path\" 2>/dev/null)\"; fi; "+
			"  _tideterm_tmux_cmd=\"\"; "+
			"  if [ -n \"$_tideterm_wsh_dir\" ]; then _tideterm_tmux_cmd=\"$_tideterm_wsh_dir/tideterm-shell\"; fi; "+
			"  if [ -z \"$_tideterm_tmux_cmd\" ]; then _tideterm_tmux_cmd=\"$HOME/%s/bin/tideterm-shell\"; fi; "+
			"  tmux set-option -g allow-passthrough on >/dev/null 2>&1 || true; "+
			"  tmux set-option -g window-size latest >/dev/null 2>&1 || true; "+
			"  tmux set-window-option -g alternate-screen off >/dev/null 2>&1 || true; "+
			"  _tideterm_term=\"${TERM-}\"; "+
			"  _tideterm_overrides_init=\"$(tmux show-option -gqv @tideterm_terminal_overrides_set 2>/dev/null)\"; "+
			"  if [ \"$_tideterm_overrides_init\" != \"1\" ]; then "+
			"    if [ -n \"$_tideterm_term\" ]; then tmux set-option -ga terminal-overrides \",${_tideterm_term}:smcup@:rmcup@\" >/dev/null 2>&1 || true; fi; "+
			"    tmux set-option -ga terminal-overrides \",xterm*:smcup@:rmcup@\" >/dev/null 2>&1 || true; "+
			"    tmux set-option -g @tideterm_terminal_overrides_set 1 >/dev/null 2>&1 || true; "+
			"  fi; "+
			"  unset _tideterm_overrides_init; "+
			"  if [ -x \"$_tideterm_tmux_cmd\" ]; then "+
			"    if tmux has-session -t %s >/dev/null 2>&1; then "+
			"      tmux set-option -t %s default-command \"$_tideterm_tmux_cmd\" >/dev/null 2>&1 || true; "+
			"      exec tmux attach -t %s; "+
			"    else "+
			"      tmux new-session -d -s %s -c \"$PWD\" \"$_tideterm_tmux_cmd\" || exit 0; "+
			"      tmux set-option -t %s default-command \"$_tideterm_tmux_cmd\" >/dev/null 2>&1 || true; "+
			"      exec tmux attach -t %s; "+
			"    fi; "+
			"  else "+
			"    exec tmux new-session -A -s %s; "+
			"  fi; "+
			"else printf \"%%s\\\\n\"%s; fi'\n",
		wavebase.RemoteWaveHomeDirName,
		sessionName,
		sessionName,
		sessionName,
		sessionName,
		sessionName,
		sessionName,
		sessionName,
		printfArgs.String(),
	)
}

func shouldAutoResumeWithTmux(cmdStr string, cmdOpts CommandOptsType) bool {
	if cmdStr != "" {
		return false
	}
	if !cmdOpts.Interactive {
		return false
	}
	enabled := wconfig.DefaultBoolPtr(wconfig.GetWatcher().GetFullConfig().Settings.TermRemoteTmuxResume, true)
	return enabled
}

func getSwapTokenBlockId(cmdOpts CommandOptsType) string {
	if cmdOpts.SwapToken == nil || cmdOpts.SwapToken.Env == nil {
		return ""
	}
	return cmdOpts.SwapToken.Env["TIDETERM_BLOCKID"]
}

func maybeAddTmuxAutoResumeToSwapToken(cmdStr string, cmdOpts CommandOptsType) {
	if !shouldAutoResumeWithTmux(cmdStr, cmdOpts) {
		return
	}
	if cmdOpts.SwapToken == nil {
		return
	}
	sessionName := tmuxSessionNameForBlockId(getSwapTokenBlockId(cmdOpts))
	if sessionName == "" {
		return
	}
	cmdOpts.SwapToken.ScriptText = makeTmuxAutoResumeShellCommand(sessionName, isChineseAppLanguage()) + cmdOpts.SwapToken.ScriptText
}

func maybeWriteTmuxAutoResumeToPty(cmdStr string, cmdOpts CommandOptsType, pty pty.Pty) {
	if !shouldAutoResumeWithTmux(cmdStr, cmdOpts) {
		return
	}
	sessionName := tmuxSessionNameForBlockId(getSwapTokenBlockId(cmdOpts))
	if sessionName == "" {
		return
	}
	_, _ = pty.Write([]byte(makeTmuxAutoResumeShellCommand(sessionName, isChineseAppLanguage())))
}

type PipePty struct {
	remoteStdinWrite *os.File
	remoteStdoutRead *os.File
}

func (pp *PipePty) Fd() uintptr {
	return pp.remoteStdinWrite.Fd()
}

func (pp *PipePty) Name() string {
	return "pipe-pty"
}

func (pp *PipePty) Read(p []byte) (n int, err error) {
	return pp.remoteStdoutRead.Read(p)
}

func (pp *PipePty) Write(p []byte) (n int, err error) {
	return pp.remoteStdinWrite.Write(p)
}

func (pp *PipePty) Close() error {
	err1 := pp.remoteStdinWrite.Close()
	err2 := pp.remoteStdoutRead.Close()

	if err1 != nil {
		return err1
	}
	return err2
}

func (pp *PipePty) WriteString(s string) (n int, err error) {
	return pp.Write([]byte(s))
}

func StartWslShellProcNoWsh(ctx context.Context, termSize waveobj.TermSize, cmdStr string, cmdOpts CommandOptsType, conn *wslconn.WslConn) (*ShellProc, error) {
	client := conn.GetClient()
	conn.Infof(ctx, "WSL-NEWSESSION (StartWslShellProcNoWsh)")

	ecmd := exec.Command("wsl.exe", "~", "-d", client.Name())

	if termSize.Rows == 0 || termSize.Cols == 0 {
		termSize.Rows = shellutil.DefaultTermRows
		termSize.Cols = shellutil.DefaultTermCols
	}
	if termSize.Rows <= 0 || termSize.Cols <= 0 {
		return nil, fmt.Errorf("invalid term size: %v", termSize)
	}
	cmdPty, err := pty.StartWithSize(ecmd, &pty.Winsize{Rows: uint16(termSize.Rows), Cols: uint16(termSize.Cols)})
	if err != nil {
		return nil, err
	}
	if cmdOpts.Cwd != "" {
		if cwdExpr := posixCwdExpr(cmdOpts.Cwd); cwdExpr != "" {
			cmdPty.Write([]byte("cd " + cwdExpr + "\n"))
		}
	}
	maybeWriteTmuxAutoResumeToPty(cmdStr, cmdOpts, cmdPty)
	cmdWrap := MakeCmdWrap(ecmd, cmdPty)
	return &ShellProc{Cmd: cmdWrap, ConnName: conn.GetName(), CloseOnce: &sync.Once{}, DoneCh: make(chan any)}, nil
}

func StartWslShellProc(ctx context.Context, termSize waveobj.TermSize, cmdStr string, cmdOpts CommandOptsType, conn *wslconn.WslConn) (*ShellProc, error) {
	client := conn.GetClient()
	conn.Infof(ctx, "WSL-NEWSESSION (StartWslShellProc)")
	connRoute := wshutil.MakeConnectionRouteId(conn.GetName())
	rpcClient := wshclient.GetBareRpcClient()
	remoteInfo, err := wshclient.RemoteGetInfoCommand(rpcClient, &wshrpc.RpcOpts{Route: connRoute, Timeout: 2000})
	if err != nil {
		return nil, fmt.Errorf("unable to obtain client info: %w", err)
	}
	log.Printf("client info collected: %+#v", remoteInfo)
	var shellPath string
	if cmdOpts.ShellPath != "" {
		conn.Infof(ctx, "using shell path from command opts: %s\n", cmdOpts.ShellPath)
		shellPath = cmdOpts.ShellPath
	}
	configShellPath := conn.GetConfigShellPath()
	if shellPath == "" && configShellPath != "" {
		conn.Infof(ctx, "using shell path from config (conn:shellpath): %s\n", configShellPath)
		shellPath = configShellPath
	}
	if shellPath == "" && remoteInfo.Shell != "" {
		conn.Infof(ctx, "using shell path detected on remote machine: %s\n", remoteInfo.Shell)
		shellPath = remoteInfo.Shell
	}
	if shellPath == "" {
		conn.Infof(ctx, "no shell path detected, using default (/bin/bash)\n")
		shellPath = "/bin/bash"
	}
	var shellOpts []string
	var cmdCombined string
	log.Printf("detected shell %q for conn %q\n", shellPath, conn.GetName())

	err = wshclient.RemoteInstallRcFilesCommand(rpcClient, &wshrpc.RpcOpts{Route: connRoute, Timeout: 2000})
	if err != nil {
		log.Printf("error installing rc files: %v", err)
		return nil, err
	}
	shellOpts = append(shellOpts, cmdOpts.ShellOpts...)
	shellType := shellutil.GetShellTypeFromShellPath(shellPath)
	conn.Infof(ctx, "detected shell type: %s\n", shellType)
	conn.Debugf(ctx, "cmdStr: %q\n", cmdStr)

	if cmdStr == "" {
		/* transform command in order to inject environment vars */
		if shellType == shellutil.ShellType_bash {
			// add --rcfile
			// cant set -l or -i with --rcfile
			bashPath := fmt.Sprintf("~/%s/%s/.bashrc", wavebase.RemoteWaveHomeDirName, shellutil.BashIntegrationDir)
			shellOpts = append(shellOpts, "--rcfile", bashPath)
		} else if shellType == shellutil.ShellType_fish {
			if cmdOpts.Login {
				shellOpts = append(shellOpts, "-l")
			}
			// source the wave.fish file
			waveFishPath := fmt.Sprintf("~/%s/%s/tideterm.fish", wavebase.RemoteWaveHomeDirName, shellutil.FishIntegrationDir)
			carg := fmt.Sprintf(`"source %s"`, waveFishPath)
			shellOpts = append(shellOpts, "-C", carg)
		} else if shellType == shellutil.ShellType_pwsh {
			pwshPath := fmt.Sprintf("~/%s/%s/tidetermpwsh.ps1", wavebase.RemoteWaveHomeDirName, shellutil.PwshIntegrationDir)
			// powershell is weird about quoted path executables and requires an ampersand first
			shellPath = "& " + shellPath
			shellOpts = append(shellOpts, "-ExecutionPolicy", "Bypass", "-NoExit", "-File", pwshPath)
		} else {
			if cmdOpts.Login {
				shellOpts = append(shellOpts, "-l")
			}
			if cmdOpts.Interactive {
				shellOpts = append(shellOpts, "-i")
			}
			// zdotdir setting moved to after session is created
		}
		cmdCombined = fmt.Sprintf("%s %s", shellPath, strings.Join(shellOpts, " "))
	} else {
		// TODO check quoting of cmdStr
		shellOpts = append(shellOpts, "-c", cmdStr)
		cmdCombined = fmt.Sprintf("%s %s", shellPath, strings.Join(shellOpts, " "))
	}
	conn.Infof(ctx, "starting shell, using command: %s\n", cmdCombined)
	conn.Infof(ctx, "WSL-NEWSESSION (StartWslShellProc)\n")

	if shellType == shellutil.ShellType_zsh {
		zshDir := fmt.Sprintf("~/%s/%s", wavebase.RemoteWaveHomeDirName, shellutil.ZshIntegrationDir)
		conn.Infof(ctx, "setting ZDOTDIR to %s\n", zshDir)
		cmdCombined = fmt.Sprintf(`ZDOTDIR=%s %s`, zshDir, cmdCombined)
	}
	packedToken, err := cmdOpts.SwapToken.PackForClient()
	if err != nil {
		conn.Infof(ctx, "error packing swap token: %v", err)
	} else {
		conn.Debugf(ctx, "packed swaptoken %s\n", packedToken)
		cmdCombined = fmt.Sprintf(`%s=%s %s`, wavebase.WaveSwapTokenVarName, packedToken, cmdCombined)
	}
	jwtToken := cmdOpts.SwapToken.Env[wavebase.WaveJwtTokenVarName]
	if jwtToken != "" {
		conn.Debugf(ctx, "adding JWT token to environment\n")
		cmdCombined = fmt.Sprintf(`%s=%s %s`, wavebase.WaveJwtTokenVarName, jwtToken, cmdCombined)
	}
	maybeAddTmuxAutoResumeToSwapToken(cmdStr, cmdOpts)
	prependCwdInitScriptToSwapToken(shellType, cmdOpts.Cwd, cmdOpts.SwapToken)
	prependHomeResetInitScriptToSwapToken(shellType, cmdOpts.SwapToken)
	log.Printf("full combined command: %s", cmdCombined)
	ecmd := exec.Command("wsl.exe", "~", "-d", client.Name(), "--", "sh", "-c", cmdCombined)
	if termSize.Rows == 0 || termSize.Cols == 0 {
		termSize.Rows = shellutil.DefaultTermRows
		termSize.Cols = shellutil.DefaultTermCols
	}
	if termSize.Rows <= 0 || termSize.Cols <= 0 {
		return nil, fmt.Errorf("invalid term size: %v", termSize)
	}
	shellutil.AddTokenSwapEntry(cmdOpts.SwapToken)
	cmdPty, err := pty.StartWithSize(ecmd, &pty.Winsize{Rows: uint16(termSize.Rows), Cols: uint16(termSize.Cols)})
	if err != nil {
		return nil, err
	}
	cmdWrap := MakeCmdWrap(ecmd, cmdPty)
	return &ShellProc{Cmd: cmdWrap, ConnName: conn.GetName(), CloseOnce: &sync.Once{}, DoneCh: make(chan any)}, nil
}

func StartRemoteShellProcNoWsh(ctx context.Context, termSize waveobj.TermSize, cmdStr string, cmdOpts CommandOptsType, conn *conncontroller.SSHConn) (*ShellProc, error) {
	session, err := conn.NewSession(ctx, "StartRemoteShellProcNoWsh")
	if err != nil {
		return nil, err
	}

	remoteStdinRead, remoteStdinWriteOurs, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	remoteStdoutReadOurs, remoteStdoutWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	pipePty := &PipePty{
		remoteStdinWrite: remoteStdinWriteOurs,
		remoteStdoutRead: remoteStdoutReadOurs,
	}
	if termSize.Rows == 0 || termSize.Cols == 0 {
		termSize.Rows = shellutil.DefaultTermRows
		termSize.Cols = shellutil.DefaultTermCols
	}
	if termSize.Rows <= 0 || termSize.Cols <= 0 {
		return nil, fmt.Errorf("invalid term size: %v", termSize)
	}
	session.Stdin = remoteStdinRead
	session.Stdout = remoteStdoutWrite
	session.Stderr = remoteStdoutWrite

	session.RequestPty("xterm-256color", termSize.Rows, termSize.Cols, nil)
	sessionWrap := MakeSessionWrap(session, "", pipePty)
	err = session.Shell()
	if err != nil {
		pipePty.Close()
		return nil, err
	}
	if cmdOpts.Cwd != "" {
		cwdExpr := posixCwdExprNoWshRemote(cmdOpts.Cwd, conn.Opts.SSHUser)
		if cwdExpr != "" {
			pipePty.WriteString("cd " + cwdExpr + "\n")
		}
	}
	maybeWriteTmuxAutoResumeToPty(cmdStr, cmdOpts, pipePty)
	return &ShellProc{Cmd: sessionWrap, ConnName: conn.GetName(), CloseOnce: &sync.Once{}, DoneCh: make(chan any)}, nil
}

func StartRemoteShellProc(ctx context.Context, logCtx context.Context, termSize waveobj.TermSize, cmdStr string, cmdOpts CommandOptsType, conn *conncontroller.SSHConn) (*ShellProc, error) {
	connRoute := wshutil.MakeConnectionRouteId(conn.GetName())
	rpcClient := wshclient.GetBareRpcClient()
	remoteInfo, err := wshclient.RemoteGetInfoCommand(rpcClient, &wshrpc.RpcOpts{Route: connRoute, Timeout: 2000})
	if err != nil {
		return nil, fmt.Errorf("unable to obtain client info: %w", err)
	}
	log.Printf("client info collected: %+#v", remoteInfo)
	var shellPath string
	if cmdOpts.ShellPath != "" {
		conn.Infof(logCtx, "using shell path from command opts: %s\n", cmdOpts.ShellPath)
		shellPath = cmdOpts.ShellPath
	}
	configShellPath := conn.GetConfigShellPath()
	if shellPath == "" && configShellPath != "" {
		conn.Infof(logCtx, "using shell path from config (conn:shellpath): %s\n", configShellPath)
		shellPath = configShellPath
	}
	if shellPath == "" && remoteInfo.Shell != "" {
		conn.Infof(logCtx, "using shell path detected on remote machine: %s\n", remoteInfo.Shell)
		shellPath = remoteInfo.Shell
	}
	if shellPath == "" {
		conn.Infof(logCtx, "no shell path detected, using default (/bin/bash)\n")
		shellPath = "/bin/bash"
	}
	var shellOpts []string
	var cmdCombined string
	log.Printf("detected shell %q for conn %q\n", shellPath, conn.GetName())
	shellOpts = append(shellOpts, cmdOpts.ShellOpts...)
	shellType := shellutil.GetShellTypeFromShellPath(shellPath)
	conn.Infof(logCtx, "detected shell type: %s\n", shellType)
	conn.Infof(logCtx, "swaptoken: %s\n", cmdOpts.SwapToken.Token)
	conn.Debugf(logCtx, "cmdStr: %q\n", cmdStr)

	if cmdStr == "" {
		/* transform command in order to inject environment vars */
		if shellType == shellutil.ShellType_bash {
			// add --rcfile
			// cant set -l or -i with --rcfile
			bashPath := fmt.Sprintf("~/%s/%s/.bashrc", wavebase.RemoteWaveHomeDirName, shellutil.BashIntegrationDir)
			shellOpts = append(shellOpts, "--rcfile", bashPath)
		} else if shellType == shellutil.ShellType_fish {
			if cmdOpts.Login {
				shellOpts = append(shellOpts, "-l")
			}
			// source the wave.fish file
			waveFishPath := fmt.Sprintf("~/%s/%s/tideterm.fish", wavebase.RemoteWaveHomeDirName, shellutil.FishIntegrationDir)
			carg := fmt.Sprintf(`"source %s"`, waveFishPath)
			shellOpts = append(shellOpts, "-C", carg)
		} else if shellType == shellutil.ShellType_pwsh {
			pwshPath := fmt.Sprintf("~/%s/%s/tidetermpwsh.ps1", wavebase.RemoteWaveHomeDirName, shellutil.PwshIntegrationDir)
			// powershell is weird about quoted path executables and requires an ampersand first
			shellPath = "& " + shellPath
			shellOpts = append(shellOpts, "-ExecutionPolicy", "Bypass", "-NoExit", "-File", pwshPath)
		} else {
			if cmdOpts.Login {
				shellOpts = append(shellOpts, "-l")
			}
			if cmdOpts.Interactive {
				shellOpts = append(shellOpts, "-i")
			}
			// zdotdir setting moved to after session is created
		}
		cmdCombined = fmt.Sprintf("%s %s", shellPath, strings.Join(shellOpts, " "))
	} else {
		// TODO check quoting of cmdStr
		shellOpts = append(shellOpts, "-c", cmdStr)
		cmdCombined = fmt.Sprintf("%s %s", shellPath, strings.Join(shellOpts, " "))
	}
	conn.Infof(logCtx, "starting shell, using command: %s\n", cmdCombined)
	session, err := conn.NewSession(logCtx, "StartRemoteShellProc")
	if err != nil {
		return nil, err
	}
	remoteStdinRead, remoteStdinWriteOurs, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	remoteStdoutReadOurs, remoteStdoutWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	pipePty := &PipePty{
		remoteStdinWrite: remoteStdinWriteOurs,
		remoteStdoutRead: remoteStdoutReadOurs,
	}
	if termSize.Rows == 0 || termSize.Cols == 0 {
		termSize.Rows = shellutil.DefaultTermRows
		termSize.Cols = shellutil.DefaultTermCols
	}
	if termSize.Rows <= 0 || termSize.Cols <= 0 {
		return nil, fmt.Errorf("invalid term size: %v", termSize)
	}
	session.Stdin = remoteStdinRead
	session.Stdout = remoteStdoutWrite
	session.Stderr = remoteStdoutWrite
	if shellType == shellutil.ShellType_zsh {
		zshDir := fmt.Sprintf("~/%s/%s", wavebase.RemoteWaveHomeDirName, shellutil.ZshIntegrationDir)
		conn.Infof(logCtx, "setting ZDOTDIR to %s\n", zshDir)
		cmdCombined = fmt.Sprintf(`ZDOTDIR=%s %s`, zshDir, cmdCombined)
	}
	packedToken, err := cmdOpts.SwapToken.PackForClient()
	if err != nil {
		conn.Infof(logCtx, "error packing swap token: %v", err)
	} else {
		conn.Debugf(logCtx, "packed swaptoken %s\n", packedToken)
		cmdCombined = fmt.Sprintf(`%s=%s %s`, wavebase.WaveSwapTokenVarName, packedToken, cmdCombined)
	}
	jwtToken := cmdOpts.SwapToken.Env[wavebase.WaveJwtTokenVarName]
	if jwtToken != "" && cmdOpts.ForceJwt {
		conn.Debugf(logCtx, "adding JWT token to environment\n")
		cmdCombined = fmt.Sprintf(`%s=%s %s`, wavebase.WaveJwtTokenVarName, jwtToken, cmdCombined)
	}
	maybeAddTmuxAutoResumeToSwapToken(cmdStr, cmdOpts)
	prependCwdInitScriptToSwapToken(shellType, cmdOpts.Cwd, cmdOpts.SwapToken)
	prependHomeResetInitScriptToSwapToken(shellType, cmdOpts.SwapToken)
	shellutil.AddTokenSwapEntry(cmdOpts.SwapToken)
	session.RequestPty("xterm-256color", termSize.Rows, termSize.Cols, nil)
	sessionWrap := MakeSessionWrap(session, cmdCombined, pipePty)
	err = sessionWrap.Start()
	if err != nil {
		pipePty.Close()
		return nil, err
	}
	return &ShellProc{Cmd: sessionWrap, ConnName: conn.GetName(), CloseOnce: &sync.Once{}, DoneCh: make(chan any)}, nil
}

func StartLocalShellProc(logCtx context.Context, termSize waveobj.TermSize, cmdStr string, cmdOpts CommandOptsType, connName string) (*ShellProc, error) {
	shellutil.InitCustomShellStartupFiles()
	var ecmd *exec.Cmd
	var shellOpts []string
	shellPath := cmdOpts.ShellPath
	if shellPath == "" {
		shellPath = shellutil.DetectLocalShellPath()
	}
	shellType := shellutil.GetShellTypeFromShellPath(shellPath)
	shellOpts = append(shellOpts, cmdOpts.ShellOpts...)
	if cmdStr == "" {
		if shellType == shellutil.ShellType_bash {
			// add --rcfile
			// cant set -l or -i with --rcfile
			shellOpts = append(shellOpts, "--rcfile", shellutil.GetLocalBashRcFileOverride())
		} else if shellType == shellutil.ShellType_fish {
			if cmdOpts.Login {
				shellOpts = append(shellOpts, "-l")
			}
			waveFishPath := shellutil.GetLocalWaveFishFilePath()
			carg := fmt.Sprintf("source %s", shellutil.HardQuoteFish(waveFishPath))
			shellOpts = append(shellOpts, "-C", carg)
		} else if shellType == shellutil.ShellType_pwsh {
			shellOpts = append(shellOpts, "-ExecutionPolicy", "Bypass", "-NoExit", "-File", shellutil.GetLocalWavePowershellEnv())
		} else {
			if cmdOpts.Login {
				shellOpts = append(shellOpts, "-l")
			}
			if cmdOpts.Interactive {
				shellOpts = append(shellOpts, "-i")
			}
		}
		blocklogger.Debugf(logCtx, "[conndebug] shell:%s shellOpts:%v\n", shellPath, shellOpts)
		ecmd = exec.Command(shellPath, shellOpts...)
		ecmd.Env = os.Environ()
		if shellType == shellutil.ShellType_zsh {
			shellutil.UpdateCmdEnv(ecmd, map[string]string{"ZDOTDIR": shellutil.GetLocalZshZDotDir()})
		}
	} else {
		shellOpts = append(shellOpts, "-c", cmdStr)
		ecmd = exec.Command(shellPath, shellOpts...)
		ecmd.Env = os.Environ()
	}

	packedToken, err := cmdOpts.SwapToken.PackForClient()
	if err != nil {
		blocklogger.Infof(logCtx, "error packing swap token: %v", err)
	} else {
		blocklogger.Debugf(logCtx, "packed swaptoken %s\n", packedToken)
		shellutil.UpdateCmdEnv(ecmd, map[string]string{wavebase.WaveSwapTokenVarName: packedToken})
	}
	jwtToken := cmdOpts.SwapToken.Env[wavebase.WaveJwtTokenVarName]
	if jwtToken != "" && cmdOpts.ForceJwt {
		blocklogger.Debugf(logCtx, "adding JWT token to environment\n")
		shellutil.UpdateCmdEnv(ecmd, map[string]string{wavebase.WaveJwtTokenVarName: jwtToken})
	}

	/*
	  For Snap installations, we need to correct the XDG environment variables as Snap
	  overrides them to point to snap directories. We will get the correct values, if
	  set, from the PAM environment. If the XDG variables are set in profile or in an
	  RC file, it will be overridden when the shell initializes.
	*/
	if os.Getenv("SNAP") != "" {
		log.Printf("Detected Snap installation, correcting XDG environment variables")
		varsToReplace := map[string]string{"XDG_CONFIG_HOME": "", "XDG_DATA_HOME": "", "XDG_CACHE_HOME": "", "XDG_RUNTIME_DIR": "", "XDG_CONFIG_DIRS": "", "XDG_DATA_DIRS": ""}
		pamEnvs := tryGetPamEnvVars()
		if len(pamEnvs) > 0 {
			// We only want to set the XDG variables from the PAM environment, all others should already be correct or may have been overridden by something else out of our control
			for k := range pamEnvs {
				if _, ok := varsToReplace[k]; ok {
					varsToReplace[k] = pamEnvs[k]
				}
			}
		}
		log.Printf("Setting XDG environment variables to: %v", varsToReplace)
		shellutil.UpdateCmdEnv(ecmd, varsToReplace)
	}

	if cmdOpts.Cwd != "" {
		ecmd.Dir = cmdOpts.Cwd
	}
	if cwdErr := checkCwd(ecmd.Dir); cwdErr != nil {
		ecmd.Dir = wavebase.GetHomeDir()
	}
	envToAdd := shellutil.WaveshellLocalEnvVars(shellutil.DefaultTermType)
	if os.Getenv("LANG") == "" {
		envToAdd["LANG"] = wavebase.DetermineLang()
	}
	shellutil.UpdateCmdEnv(ecmd, envToAdd)
	if termSize.Rows == 0 || termSize.Cols == 0 {
		termSize.Rows = shellutil.DefaultTermRows
		termSize.Cols = shellutil.DefaultTermCols
	}
	if termSize.Rows <= 0 || termSize.Cols <= 0 {
		return nil, fmt.Errorf("invalid term size: %v", termSize)
	}
	shellutil.AddTokenSwapEntry(cmdOpts.SwapToken)
	cmdPty, err := pty.StartWithSize(ecmd, &pty.Winsize{Rows: uint16(termSize.Rows), Cols: uint16(termSize.Cols)})
	if err != nil {
		return nil, err
	}
	cmdWrap := MakeCmdWrap(ecmd, cmdPty)
	return &ShellProc{Cmd: cmdWrap, ConnName: connName, CloseOnce: &sync.Once{}, DoneCh: make(chan any)}, nil
}

func RunSimpleCmdInPty(ecmd *exec.Cmd, termSize waveobj.TermSize) ([]byte, error) {
	ecmd.Env = os.Environ()
	shellutil.UpdateCmdEnv(ecmd, shellutil.WaveshellLocalEnvVars(shellutil.DefaultTermType))
	if termSize.Rows == 0 || termSize.Cols == 0 {
		termSize.Rows = shellutil.DefaultTermRows
		termSize.Cols = shellutil.DefaultTermCols
	}
	if termSize.Rows <= 0 || termSize.Cols <= 0 {
		return nil, fmt.Errorf("invalid term size: %v", termSize)
	}
	cmdPty, err := pty.StartWithSize(ecmd, &pty.Winsize{Rows: uint16(termSize.Rows), Cols: uint16(termSize.Cols)})
	if err != nil {
		cmdPty.Close()
		return nil, err
	}
	if runtime.GOOS != "windows" {
		defer cmdPty.Close()
	}
	ioDone := make(chan bool)
	var outputBuf bytes.Buffer
	go func() {
		panichandler.PanicHandler("RunSimpleCmdInPty:ioCopy", recover())
		// ignore error (/dev/ptmx has read error when process is done)
		defer close(ioDone)
		io.Copy(&outputBuf, cmdPty)
	}()
	exitErr := ecmd.Wait()
	if exitErr != nil {
		return nil, exitErr
	}
	<-ioDone
	return outputBuf.Bytes(), nil
}

const etcEnvironmentPath = "/etc/environment"
const etcSecurityPath = "/etc/security/pam_env.conf"
const userEnvironmentPath = "~/.pam_environment"

var pamParseOpts *pamparse.PamParseOpts = pamparse.ParsePasswdSafe()

/*
tryGetPamEnvVars tries to get the environment variables from /etc/environment,
/etc/security/pam_env.conf, and ~/.pam_environment.

It then returns a map of the environment variables, overriding duplicates with
the following order of precedence:
1. /etc/environment
2. /etc/security/pam_env.conf
3. ~/.pam_environment
*/
func tryGetPamEnvVars() map[string]string {
	envVars, err := pamparse.ParseEnvironmentFile(etcEnvironmentPath)
	if err != nil {
		log.Printf("error parsing %s: %v", etcEnvironmentPath, err)
	}
	envVars2, err := pamparse.ParseEnvironmentConfFile(etcSecurityPath, pamParseOpts)
	if err != nil {
		log.Printf("error parsing %s: %v", etcSecurityPath, err)
	}
	envVars3, err := pamparse.ParseEnvironmentConfFile(wavebase.ExpandHomeDirSafe(userEnvironmentPath), pamParseOpts)
	if err != nil {
		log.Printf("error parsing %s: %v", userEnvironmentPath, err)
	}
	maps.Copy(envVars, envVars2)
	maps.Copy(envVars, envVars3)
	if runtime_dir, ok := envVars["XDG_RUNTIME_DIR"]; !ok || runtime_dir == "" {
		envVars["XDG_RUNTIME_DIR"] = "/run/user/" + fmt.Sprint(os.Getuid())
	}
	return envVars
}
