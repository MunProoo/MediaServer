package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 개발 환경에서는 모든 origin 허용
	},
}

// HealthStatus 서버 상태 정보
type HealthStatus struct {
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Uptime    int64     `json:"uptime"`            // 초 단위
	Streams   []Stream  `json:"streams,omitempty"` // 스트림 목록
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

var serverStartTime = time.Now()
var appConfig *Config

func StartServer() {
	// Config 파일 로드
	var err error
	appConfig, err = loadConfig("config.json")
	mediaServerClient = getMediaServerClient()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Config loaded: Client Port=%d, MediaServer=%s:%d, TurnServer Port=%d",
		appConfig.Server.ClientPort,
		appConfig.Server.MediaServer.Address,
		appConfig.Server.MediaServer.Port,
		appConfig.Server.TurnServer.Port)

	// Gin 라우터 생성
	r := gin.Default()

	// 정적 파일 서빙
	r.Static("/static", "./static")
	r.StaticFile("/favicon.ico", "./static/favicon.ico")

	// HTML 템플릿 로드
	r.LoadHTMLGlob("templates/*")

	// 메인 페이지
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "Media Server Client",
		})
	})

	// API 엔드포인트
	r.GET("/v1/coreLiveView", HTTPAPICoreLiveView)

	// 녹화 영상 페이지 - index.html로 리다이렉트 (탭으로 표시)
	r.GET("/v1/recording", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/?tab=recording")
	})

	// Stream 관리 API
	api := r.Group("/api")
	{
		api.GET("/streams", getStreamList)
		api.GET("/streams/:streamID", getStream)
		api.POST("/streams", addStream)
		api.PUT("/streams/:streamID", updateStream)
		api.DELETE("/streams/:streamID", deleteStream)
		api.POST("/streams/:streamID/recording", updateStreamRecording)
	}

	// Proxy 엔드포인트 - 모든 메서드, 모든 경로 처리
	r.Any("/proxy/*path", proxyHandler)

	// WebSocket health 체크 엔드포인트
	r.GET("/ws/health", handleWebSocketHealth)

	// 서버 시작
	port := appConfig.Server.ClientPort
	log.Printf("Client server starting on port :%d", port)
	if err := r.Run(fmt.Sprintf(":%d", port)); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

// loadConfig config.json 파일 로드
func loadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
