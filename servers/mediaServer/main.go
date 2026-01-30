package main

import (
	"log"
)

func StartServer() {
	log.Printf("[INFO] media server initializing: ver=%s", "1.0.0")

	Storage = NewStreamCore()

	// 모니터링 시스템 초기화
	InitMonitoring()

	go HTTPAPIServer()
	// RTSP 서버
	go RTSPServer()
	go Storage.StreamChannelRunAll()

	// 유지보수 매니저 (자정 녹화 재시작, 녹화 디스크 정리 ..)
	go Storage.MaintenanceManager()

	// Windows 서비스 모드에서는 시그널 대기 없이 바로 리턴
	log.Printf("[INFO] [main] [StartServer] Server started in service")
	return
}
