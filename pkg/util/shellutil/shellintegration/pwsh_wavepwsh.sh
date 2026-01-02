# We source this file with -NoExit -File
$env:PATH = {{.WSHBINDIR_PWSH}} + "{{.PATHSEP}}" + $env:PATH

# Source dynamic script from wsh token
if ($env:TIDETERM_SWAPTOKEN) {
    $tideterm_swaptoken_output = wsh token $env:TIDETERM_SWAPTOKEN pwsh 2>$null | Out-String
    if ($tideterm_swaptoken_output -and $tideterm_swaptoken_output -ne "") {
        Invoke-Expression $tideterm_swaptoken_output
    }
}
Remove-Variable -Name tideterm_swaptoken_output -ErrorAction SilentlyContinue
Remove-Item Env:TIDETERM_SWAPTOKEN -ErrorAction SilentlyContinue

# Load TideTerm completions
wsh completion powershell | Out-String | Invoke-Expression

if ($PSVersionTable.PSVersion.Major -lt 7) {
    return  # skip OSC setup entirely
}

$Global:_TIDETERM_SI_FIRSTPROMPT = $true

# shell integration
function Global:_waveterm_si_blocked {
    # GNU Screen sets TERM=screen* and STY; in tmux TERM is often screen-256color, so only block
    # "screen*" when we're not actually inside tmux.
    $inScreen = -not [string]::IsNullOrEmpty($env:STY)
    $termIsScreen = $env:TERM -like "screen*"
    $inTmux = -not [string]::IsNullOrEmpty($env:TMUX)
    return ($inScreen -or ($termIsScreen -and -not $inTmux))
}

function Global:_waveterm_si_in_tmux {
    return ($env:TMUX -or $env:TERM -like "tmux*")
}

function Global:_waveterm_si_write([string]$text) {
    if (_waveterm_si_in_tmux) {
        Write-Host -NoNewline ("`ePtmux;`e" + $text + "`e\\")
    } else {
        Write-Host -NoNewline $text
    }
}

function Global:_waveterm_si_osc7 {
    if (_waveterm_si_blocked) { return }
    
    # Percent-encode the raw path as-is (handles UNC, drive letters, etc.)
    $encoded_pwd = [System.Uri]::EscapeDataString($PWD.Path)
    
    # OSC 7 - current directory
    _waveterm_si_write "`e]7;file://localhost/$encoded_pwd`a"
}

function Global:_waveterm_si_prompt {
    if (_waveterm_si_blocked) { return }
    
    if ($Global:_TIDETERM_SI_FIRSTPROMPT) {
		# not sending uname
		       $shellversion = $PSVersionTable.PSVersion.ToString()
		       _waveterm_si_write "`e]16162;M;{`"shell`":`"pwsh`",`"shellversion`":`"$shellversion`",`"integration`":false}`a"
        $Global:_TIDETERM_SI_FIRSTPROMPT = $false
    }
    
    _waveterm_si_osc7
}

# Add the OSC 7 call to the prompt function
if (Test-Path Function:\prompt) {
    $global:_waveterm_original_prompt = $function:prompt
    function Global:prompt {
        _waveterm_si_prompt
        & $global:_waveterm_original_prompt
    }
} else {
    function Global:prompt {
        _waveterm_si_prompt
        "PS $($executionContext.SessionState.Path.CurrentLocation)$('>' * ($nestedPromptLevel + 1)) "
    }
}
