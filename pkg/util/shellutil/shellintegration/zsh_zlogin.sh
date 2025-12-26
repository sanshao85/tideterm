# Source the original zlogin
[ -f ~/.zlogin ] && source ~/.zlogin

# Unset ZDOTDIR only if it hasn't been modified
if [ "$ZDOTDIR" = "$TIDETERM_ZDOTDIR" ]; then
  unset ZDOTDIR
fi
