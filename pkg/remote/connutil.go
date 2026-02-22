// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package remote

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/sanshao85/tideterm/pkg/blocklogger"
	"github.com/sanshao85/tideterm/pkg/genconn"
	"github.com/sanshao85/tideterm/pkg/remote/awsconn"
	"github.com/sanshao85/tideterm/pkg/util/iterfn"
	"github.com/sanshao85/tideterm/pkg/util/shellutil"
	"github.com/sanshao85/tideterm/pkg/wavebase"
	"github.com/sanshao85/tideterm/pkg/wconfig"
	"golang.org/x/crypto/ssh"
)

var userHostRe = regexp.MustCompile(`^([a-zA-Z0-9][a-zA-Z0-9._@\\-]*@)?([a-zA-Z0-9][a-zA-Z0-9.-]*)(?::([0-9]+))?$`)

func ParseOpts(input string) (*SSHOpts, error) {
	m := userHostRe.FindStringSubmatch(input)
	if m == nil {
		return nil, fmt.Errorf("invalid format of user@host argument")
	}
	remoteUser, remoteHost, remotePort := m[1], m[2], m[3]
	remoteUser = strings.Trim(remoteUser, "@")

	return &SSHOpts{SSHHost: remoteHost, SSHUser: remoteUser, SSHPort: remotePort}, nil
}

func normalizeOs(os string) string {
	os = strings.ToLower(strings.TrimSpace(os))
	return os
}

func normalizeArch(arch string) string {
	arch = strings.ToLower(strings.TrimSpace(arch))
	switch arch {
	case "x86_64", "amd64":
		arch = "x64"
	case "arm64", "aarch64":
		arch = "arm64"
	}
	return arch
}

var ansiEscapeRe = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")

func stripANSIEscapes(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}

func parseClientPlatformFromUnameOutput(output string) (string, string, error) {
	output = strings.ReplaceAll(output, "\r\n", "\n")
	output = strings.ReplaceAll(output, "\r", "\n")
	output = stripANSIEscapes(output)

	// First try the full output (common case).
	parts := strings.Fields(strings.ToLower(strings.TrimSpace(output)))
	if len(parts) == 2 {
		osName, archName := normalizeOs(parts[0]), normalizeArch(parts[1])
		if err := wavebase.ValidateWshSupportedArch(osName, archName); err == nil {
			return osName, archName, nil
		}
	}

	// Some environments print banners/MOTD/rcfile output before the command, which can pollute stdout.
	// Scan line-by-line from the end and pick the first valid (os, arch) pair.
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		fields := strings.Fields(strings.ToLower(line))
		if len(fields) != 2 {
			continue
		}
		osName, archName := normalizeOs(fields[0]), normalizeArch(fields[1])
		if err := wavebase.ValidateWshSupportedArch(osName, archName); err == nil {
			return osName, archName, nil
		}
	}

	return "", "", fmt.Errorf("unexpected output from uname: %s", strings.TrimSpace(output))
}

// returns (os, arch, error)
// guaranteed to return a supported platform
func GetClientPlatform(ctx context.Context, shell genconn.ShellClient) (string, string, error) {
	blocklogger.Infof(ctx, "[conndebug] running `uname -sm` to detect client platform\n")
	stdout, stderr, err := genconn.RunSimpleCommand(ctx, shell, genconn.CommandSpec{
		Cmd: "uname -sm",
	})
	if err != nil {
		return "", "", fmt.Errorf("error running uname -sm: %w, stderr: %s", err, stderr)
	}
	osName, archName, err := parseClientPlatformFromUnameOutput(stdout)
	if err != nil {
		return "", "", err
	}
	return osName, archName, nil
}

func GetClientPlatformFromOsArchStr(ctx context.Context, osArchStr string) (string, string, error) {
	osName, archName, err := parseClientPlatformFromUnameOutput(osArchStr)
	if err != nil {
		return "", "", err
	}
	return osName, archName, nil
}

var installTemplateRawDefault = strings.TrimSpace(`
_tideterm_user="$(id -un 2>/dev/null || true)";
_tideterm_home="";
if [ -n "$_tideterm_user" ]; then
  if command -v getent >/dev/null 2>&1; then
    _tideterm_home="$(getent passwd "$_tideterm_user" | cut -d: -f6 2>/dev/null || true)";
  fi;
  if [ -z "$_tideterm_home" ]; then
    _tideterm_home="$(eval echo ~$_tideterm_user 2>/dev/null || true)";
  fi;
fi;
if [ -z "$_tideterm_home" ]; then _tideterm_home="$HOME"; fi;

install_dir="$_tideterm_home/{{.waveHomeDirName}}/bin";
install_path="$install_dir/wsh";
temp_path="$install_path.temp";

mkdir -p "$install_dir" || exit 1;
cat > "$temp_path" || exit 1;
mv "$temp_path" "$install_path" || exit 1;
chmod a+x "$install_path" || exit 1;
`)
var installTemplate = template.Must(template.New("wsh-install-template").Parse(installTemplateRawDefault))

func CpWshToRemote(ctx context.Context, client *ssh.Client, clientOs string, clientArch string) error {
	deadline, ok := ctx.Deadline()
	if ok {
		blocklogger.Debugf(ctx, "[conndebug] CpWshToRemote, timeout: %v\n", time.Until(deadline))
	}
	wshLocalPath, err := shellutil.GetLocalWshBinaryPath(wavebase.WaveVersion, clientOs, clientArch)
	if err != nil {
		return err
	}
	input, err := os.Open(wshLocalPath)
	if err != nil {
		return fmt.Errorf("cannot open local file %s: %w", wshLocalPath, err)
	}
	defer input.Close()
	installWords := map[string]string{
		"waveHomeDirName": wavebase.RemoteWaveHomeDirName,
	}
	var installCmd bytes.Buffer
	if err := installTemplate.Execute(&installCmd, installWords); err != nil {
		return fmt.Errorf("failed to prepare install command: %w", err)
	}
	blocklogger.Infof(ctx, "[conndebug] copying %q to remote server %q\n", wshLocalPath, wavebase.RemoteFullWshBinPath)
	genCmd, err := genconn.MakeSSHCmdClient(client, genconn.CommandSpec{
		Cmd: installCmd.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to create remote command: %w", err)
	}
	stdin, err := genCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	defer stdin.Close()
	stderrBuf, err := genconn.MakeStderrSyncBuffer(genCmd)
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}
	if err := genCmd.Start(); err != nil {
		return fmt.Errorf("failed to start remote command: %w", err)
	}
	copyDone := make(chan error, 1)
	go func() {
		defer close(copyDone)
		defer stdin.Close()
		if _, err := io.Copy(stdin, input); err != nil && err != io.EOF {
			copyDone <- fmt.Errorf("failed to copy data: %w", err)
		} else {
			copyDone <- nil
		}
	}()
	procErr := genconn.ProcessContextWait(ctx, genCmd)
	if procErr != nil {
		return fmt.Errorf("remote command failed: %w (stderr: %s)", procErr, stderrBuf.String())
	}
	copyErr := <-copyDone
	if copyErr != nil {
		return fmt.Errorf("failed to copy data: %w (stderr: %s)", copyErr, stderrBuf.String())
	}
	return nil
}

func IsPowershell(shellPath string) bool {
	// get the base path, and then check contains
	shellBase := filepath.Base(shellPath)
	return strings.Contains(shellBase, "powershell") || strings.Contains(shellBase, "pwsh")
}

func NormalizeConfigPattern(pattern string) string {
	userName, err := WaveSshConfigUserSettings().GetStrict(pattern, "User")
	if err != nil || userName == "" {
		log.Printf("warning: error parsing username of %s for conn dropdown: %v", pattern, err)
		localUser, err := user.Current()
		if err == nil {
			userName = localUser.Username
		}
	}
	port, err := WaveSshConfigUserSettings().GetStrict(pattern, "Port")
	if err != nil {
		port = "22"
	}
	if userName != "" {
		userName += "@"
	}
	if port == "22" {
		port = ""
	} else {
		port = ":" + port
	}
	return fmt.Sprintf("%s%s%s", userName, pattern, port)
}

func ParseProfiles() []string {
	connfile, cerrs := wconfig.ReadWaveHomeConfigFile(wconfig.ProfilesFile)
	if len(cerrs) > 0 {
		log.Printf("error reading config file: %v", cerrs[0])
		return nil
	}

	awsProfiles := awsconn.ParseProfiles()
	for profile := range awsProfiles {
		connfile[profile] = struct{}{}
	}

	return iterfn.MapKeysToSorted(connfile)
}
