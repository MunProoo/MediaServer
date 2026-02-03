package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// handleWebSocketHealth WebSocket을 통한 health 체크 핸들러
func handleWebSocketHealth(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	sendHealthStatus(conn)

	log.Println("WebSocket client connected for health check")

	// 주기적으로 health 상태 전송 (5초마다)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Ping/Pong 핸들러
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Ping 전송 (30초마다)
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// 메시지 수신 루프
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket error: %v", err)
				}
				break
			}
			log.Printf("Received message: %s", message)
		}
	}()

	// Health 상태 전송 루프
	for {
		select {
		case <-ticker.C:
			sendHealthStatus(conn)
		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("Failed to send ping: %v", err)
				return
			}
		}
	}
}

func sendHealthStatus(conn *websocket.Conn) {
	uptime := int64(time.Since(serverStartTime).Seconds())
	mediaServer := fmt.Sprintf("https://%s:%d", appConfig.Server.MediaServer.Address, appConfig.Server.MediaServer.Port)
	status := HealthStatus{
		Status:      "ok",
		Message:     "Client server is running",
		Timestamp:   time.Now(),
		Uptime:      uptime,
		Streams:     appConfig.StreamList, // 스트림 목록 포함
		MediaServer: mediaServer,
	}

	data, err := json.Marshal(status)
	if err != nil {
		log.Printf("Failed to marshal status: %v", err)
		return
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("Failed to write message: %v", err)
	}
}
