# this file is sourced with -C
# Add TideTerm binary directory to PATH
set -x PATH {{.WSHBINDIR}} $PATH

# Source dynamic script from wsh token (the echo is to prevent fish from complaining about empty input)
if set -q TIDETERM_SWAPTOKEN
    wsh token "$TIDETERM_SWAPTOKEN" fish 2>/dev/null | source
    set -e TIDETERM_SWAPTOKEN
end

# Load TideTerm completions
wsh completion fish | source

set -g _TIDETERM_SI_FIRSTPROMPT 1

# shell integration
function _waveterm_si_blocked
    # GNU Screen sets TERM=screen* and STY; in tmux TERM is often screen-256color, so only block
    # "screen*" when we're not actually inside tmux.
    set -q STY; and return 0
    if string match -q 'screen*' -- $TERM
        set -q TMUX; or return 0
    end
    return 1
end

function _waveterm_si_in_tmux
    set -q TMUX; or string match -q 'tmux*' -- $TERM
end

function _waveterm_si_write
    if _waveterm_si_in_tmux
        printf '\033Ptmux;\033'
        printf $argv
        printf '\033\\'
    else
        printf $argv
    end
end

function _waveterm_si_osc7
    _waveterm_si_blocked; and return
    # Use fish-native URL encoding
    set -l encoded_pwd (string escape --style=url -- "$PWD")
    _waveterm_si_write '\033]7;file://localhost%s\007' $encoded_pwd
end

function _waveterm_si_prompt --on-event fish_prompt
    set -l _waveterm_si_status $status
    _waveterm_si_blocked; and return
    if test $_TIDETERM_SI_FIRSTPROMPT -eq 1
        set -l uname_info (uname -smr 2>/dev/null)
        _waveterm_si_write '\033]16162;M;{"shell":"fish","shellversion":"%s","uname":"%s","integration":true}\007' $FISH_VERSION "$uname_info"
        # OSC 7 only sent on first prompt - chpwd hook handles directory changes
        _waveterm_si_osc7
    else
        _waveterm_si_write '\033]16162;D;{"exitcode":%d}\007' $_waveterm_si_status
    end
    _waveterm_si_write '\033]16162;A\007'
    set -g _TIDETERM_SI_FIRSTPROMPT 0
end

function _waveterm_si_preexec --on-event fish_preexec
    _waveterm_si_blocked; and return
    set -l cmd (string join -- ' ' $argv)
    set -l cmd_length (string length -- "$cmd")
    if test $cmd_length -gt 8192
        set -l cmd64 (printf '# command too large (%d bytes)' $cmd_length | base64 2>/dev/null | string replace -a '\n' '' | string replace -a '\r' '')
        _waveterm_si_write '\033]16162;C;{"cmd64":"%s"}\007' "$cmd64"
    else
        set -l cmd64 (printf '%s' "$cmd" | base64 2>/dev/null | string replace -a '\n' '' | string replace -a '\r' '')
        if test -n "$cmd64"
            _waveterm_si_write '\033]16162;C;{"cmd64":"%s"}\007' "$cmd64"
        else
            _waveterm_si_write '\033]16162;C\007'
        end
    end
end

# Also update on directory change
function _waveterm_si_chpwd --on-variable PWD
    _waveterm_si_osc7
end
