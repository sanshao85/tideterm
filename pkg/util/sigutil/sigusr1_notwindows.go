//go:build !windows

// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package sigutil

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sanshao85/tideterm/pkg/panichandler"
	"github.com/sanshao85/tideterm/pkg/util/utilfn"
)

func InstallSIGUSR1Handler() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGUSR1)
	go func() {
		defer func() {
			panichandler.PanicHandler("InstallSIGUSR1Handler", recover())
		}()
		for range sigCh {
			utilfn.DumpGoRoutineStacks()
		}
	}()
}
