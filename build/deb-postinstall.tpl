#!/bin/bash

if type update-alternatives 2>/dev/null >&1; then
    # Remove previous link if it doesn't use update-alternatives
    if [ -L '/usr/bin/tideterm' -a -e '/usr/bin/tideterm' -a "`readlink '/usr/bin/tideterm'`" != '/etc/alternatives/tideterm' ]; then
        rm -f '/usr/bin/tideterm'
    fi
    update-alternatives --install '/usr/bin/tideterm' 'tideterm' '/opt/TideTerm/tideterm' 100 || ln -sf '/opt/TideTerm/tideterm' '/usr/bin/tideterm'
else
    ln -sf '/opt/TideTerm/tideterm' '/usr/bin/tideterm'
fi

chmod 4755 '/opt/TideTerm/chrome-sandbox' || true

if hash update-mime-database 2>/dev/null; then
    update-mime-database /usr/share/mime || true
fi

if hash update-desktop-database 2>/dev/null; then
    update-desktop-database /usr/share/applications || true
fi
