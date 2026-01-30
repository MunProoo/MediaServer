//go:build linux || darwin || freebsd || netbsd || openbsd
// +build linux darwin freebsd netbsd openbsd

package logUtil

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

var sigChan chan os.Signal = nil

func InitLinuxSignalManager() {

	if nil != sigChan {
		return
	}

	sigChan = make(chan os.Signal, 100)

	signal.Notify(sigChan, syscall.SIGPIPE, syscall.SIGUSR1, syscall.SIGUSR2)

	go func() {
		for {
			sig := <-sigChan
			log.Printf("[INFO] [linux] [InitLinuxSignalManager] [Signal] %s", sig)
		}
	}()
}
