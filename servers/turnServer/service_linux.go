//go:build linux || darwin || freebsd || netbsd || openbsd
// +build linux darwin freebsd netbsd openbsd

package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"mjy/logUtil"
)

func main() {
	logUtil.InitLinuxSignalManager()

	if err := logUtil.InitLogging("turnServer", true); err != nil {
		log.Printf("[ERROR] [main] Failed to initialize logging: %v", err)
	}
	os.Chdir(filepath.Dir(os.Args[0]))

	StartServer()

	// Block until user sends SIGINT or SIGTERM
	// SIGINT : 터미널의 Ctrl + C
	// SIGTERM : 프로세스 종료 신호 (kill process)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	log.Printf("[INFO] [main] [main] Signal received, shutting down...")
	CloseServer()
}
