# Store the initial ZDOTDIR value
TIDETERM_ZDOTDIR="$ZDOTDIR"

# Source the original zshenv
[ -f ~/.zshenv ] && source ~/.zshenv

# Detect if ZDOTDIR has changed
if [ "$ZDOTDIR" != "$TIDETERM_ZDOTDIR" ]; then
  # If changed, manually source your custom zshrc from the original TIDETERM_ZDOTDIR
  [ -f "$TIDETERM_ZDOTDIR/.zshrc" ] && source "$TIDETERM_ZDOTDIR/.zshrc"
fi
