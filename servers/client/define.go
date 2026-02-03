package main

import "time"

const (
	METHOD_ADD    = 1
	METHOD_EDIT   = 2
	METHOD_DELETE = 3
)

// HealthStatus 서버 상태 정보
type HealthStatus struct {
	Status      string    `json:"status"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	Uptime      int64     `json:"uptime"`            // 초 단위
	Streams     []Stream  `json:"streams,omitempty"` // 스트림 목록
	MediaServer string    `json:"mediaServer"`
}

// Config 전체 설정 구조체
type Config struct {
	Server     ServerConfig `json:"server"`
	StreamList []Stream     `json:"streamList"`
}

// ServerConfig 서버 설정
type ServerConfig struct {
	ClientPort  int               `json:"clientPort"`
	MediaServer MediaServerConfig `json:"mediaServer"`
	TurnServer  TurnServerConfig  `json:"turnServer"`
}

// MediaServerConfig Media Server 설정
type MediaServerConfig struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// TurnServerConfig TURN Server 설정
type TurnServerConfig struct {
	Port int `json:"port"`
}

// Stream 스트림 정보
type Stream struct {
	Name      string `json:"name"`
	IP        string `json:"ip"`
	RtspURL   string `json:"rtspURL"`
	StreamID  string `json:"streamID"`
	Recording bool   `json:"recording"`
}
