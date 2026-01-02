
# Source /etc/profile if it exists
if [ -f /etc/profile ]; then
    . /etc/profile
fi

_waveterm_si_reset_home() {
    # Ensure HOME matches the passwd entry for the user.
    # Some environments (containers / custom profiles) override HOME unexpectedly (e.g. to /tmp),
    # which breaks "~" expansion for users in TideTerm terminals.
    local _waveterm_user
    _waveterm_user="$(id -un 2>/dev/null)"
    if [ -z "$_waveterm_user" ]; then
        return 0
    fi
    local _waveterm_home
    _waveterm_home="$(eval echo ~${_waveterm_user} 2>/dev/null)"
    if [ -n "$_waveterm_home" ] && [ "$_waveterm_home" != "$HOME" ]; then
        export HOME="$_waveterm_home"
    fi
}

_waveterm_si_reset_home

TIDETERM_WSHBINDIR={{.WSHBINDIR}}

# after /etc/profile which is likely to clobber the path
export PATH="$TIDETERM_WSHBINDIR:$PATH"

# Source the dynamic script from wsh token
if [ -n "${TIDETERM_SWAPTOKEN-}" ]; then
    eval "$(wsh token "$TIDETERM_SWAPTOKEN" bash 2> /dev/null)"
    unset TIDETERM_SWAPTOKEN
fi

# Source the first of ~/.bash_profile, ~/.bash_login, or ~/.profile that exists
if [ -f ~/.bash_profile ]; then
    . ~/.bash_profile
elif [ -f ~/.bash_login ]; then
    . ~/.bash_login
elif [ -f ~/.profile ]; then
    . ~/.profile
fi

_waveterm_si_reset_home
unset -f _waveterm_si_reset_home

if [[ ":$PATH:" != *":$TIDETERM_WSHBINDIR:"* ]]; then
    export PATH="$TIDETERM_WSHBINDIR:$PATH"
fi
unset TIDETERM_WSHBINDIR
if type _init_completion &>/dev/null; then
  source <(wsh completion bash)
fi

# extdebug breaks bash-preexec semantics; bail out cleanly
if shopt -q extdebug; then
  # printf 'wave si: disabled (bash extdebug enabled)\n' >&2
  printf '\033]16162;M;{"integration":false}\007'
  return 0
fi

# Source bash-preexec for proper preexec/precmd hook support
if [ -z "${bash_preexec_imported:-}" ]; then
    _TIDETERM_SI_BASHRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    if [ -f "$_TIDETERM_SI_BASHRC_DIR/bash_preexec.sh" ]; then
        source "$_TIDETERM_SI_BASHRC_DIR/bash_preexec.sh"
    fi
    unset _TIDETERM_SI_BASHRC_DIR
fi

# Check if bash-preexec was successfully imported
if [ -z "${bash_preexec_imported:-}" ]; then
    # bash-preexec failed to import, disable shell integration
    printf '\033]16162;M;{"integration":false}\007'
    return 0
fi

_TIDETERM_SI_FIRSTPROMPT=1

# TideTerm Shell Integration
_waveterm_si_blocked() {
    # GNU Screen sets TERM=screen* and STY; in tmux TERM is often screen-256color, so only block
    # "screen*" when we're not actually inside tmux.
    [[ -n "$STY" || ( "$TERM" == screen* && -z "${TMUX-}" ) ]]
}

_waveterm_si_in_tmux() {
    [[ -n "$TMUX" || "$TERM" == tmux* ]]
}

_waveterm_si_write() {
    if _waveterm_si_in_tmux; then
        printf '\033Ptmux;\033'
        printf "$@"
        printf '\033\\'
    else
        printf "$@"
    fi
}

_waveterm_si_urlencode() {
    local s="$1"
    s="${s//%/%25}"
    s="${s// /%20}"
    s="${s//#/%23}"
    s="${s//\?/%3F}"
    s="${s//&/%26}"
    s="${s//;/%3B}"
    s="${s//+/%2B}"
    printf '%s' "$s"
}

_waveterm_si_osc7() {
    _waveterm_si_blocked && return
    local encoded_pwd=$(_waveterm_si_urlencode "$PWD")
    _waveterm_si_write '\033]7;file://localhost%s\007' "$encoded_pwd"
}

_waveterm_si_precmd() {
    local _waveterm_si_status=$?
    _waveterm_si_blocked && return
    
    if [ "$_TIDETERM_SI_FIRSTPROMPT" -eq 1 ]; then
        local uname_info
        uname_info=$(uname -smr 2>/dev/null)
        _waveterm_si_write '\033]16162;M;{"shell":"bash","shellversion":"%s","uname":"%s","integration":true}\007' "$BASH_VERSION" "$uname_info"
    else
        _waveterm_si_write '\033]16162;D;{"exitcode":%d}\007' "$_waveterm_si_status"
    fi
    # OSC 7 sent on every prompt - bash has no chpwd hook for directory changes
    _waveterm_si_osc7
    _waveterm_si_write '\033]16162;A\007'
    _TIDETERM_SI_FIRSTPROMPT=0
}

_waveterm_si_preexec() {
    _waveterm_si_blocked && return
    
    local cmd="$1"
    local cmd_length=${#cmd}
    if [ "$cmd_length" -gt 8192 ]; then
        cmd=$(printf '# command too large (%d bytes)' "$cmd_length")
    fi
    local cmd64
    cmd64=$(printf '%s' "$cmd" | base64 2>/dev/null | tr -d '\n\r')
    if [ -n "$cmd64" ]; then
        _waveterm_si_write '\033]16162;C;{"cmd64":"%s"}\007' "$cmd64"
    else
        _waveterm_si_write '\033]16162;C\007'
    fi
}

# Add our functions to the bash-preexec arrays
precmd_functions+=(_waveterm_si_precmd)
preexec_functions+=(_waveterm_si_preexec)
