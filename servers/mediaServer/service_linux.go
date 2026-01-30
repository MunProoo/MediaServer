//go:build linux || darwin || freebsd || netbsd || openbsd
// +build linux darwin freebsd netbsd openbsd

package main

import (
	"log"
	"mjy/logUtil"
	"os"
	"path/filepath"
)

func main() {

	logUtil.InitLinuxSignalManager()

	os.Chdir(filepath.Dir(os.Args[0]))

	// 로그 초기화 (Linux는 일반적으로 데몬 모드이므로 false로 설정)
	// 표준 출력이 있으면 콘솔에도 출력, 없으면 파일만
	if err := logUtil.InitLogging("mediaServer", true); err != nil {
		log.Printf("[ERROR] [main] Failed to initialize logging: %v", err)
		// 로그 초기화 실패해도 서비스는 계속 실행
	}
	defer logUtil.CloseLogging()

	StartServer()
}
