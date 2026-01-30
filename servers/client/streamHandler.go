package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// getStreamList 스트림 목록 반환
func getStreamList(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"streams": appConfig.StreamList,
	})
}

// getStream 특정 스트림 조회
func getStream(c *gin.Context) {
	streamID := c.Param("streamID")
	for _, stream := range appConfig.StreamList {
		if stream.StreamID == streamID {
			c.JSON(http.StatusOK, gin.H{
				"stream": stream,
			})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{
		"message": "Stream not found",
	})
}

// addStream 스트림 추가
func addStream(c *gin.Context) {
	var newStream Stream
	if err := c.ShouldBindJSON(&newStream); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid request data",
			"error":   err.Error(),
		})
		return
	}

	// ID가 없으면 생성
	newStream.StreamID = CreateUUID()

	// recording은 기본값 false로 설정 (추가 후 별도 API로 설정)
	newStream.Recording = false

	result := streamEditReq(newStream, METHOD_ADD)
	if !result {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to add stream",
			"error":   "Failed to add stream",
		})
		return
	}

	appConfig.StreamList = append(appConfig.StreamList, newStream)

	if err := saveConfig("config.json", appConfig); err != nil {
		log.Printf("Failed to save config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to save stream",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Stream added successfully",
		"stream":  newStream,
	})
}

// updateStream 스트림 수정 (recording 제외)
func updateStream(c *gin.Context) {
	streamID := c.Param("streamID")
	var updatedStream Stream
	if err := c.ShouldBindJSON(&updatedStream); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid request data",
			"error":   err.Error(),
		})
		return
	}

	// ID 일치 확인
	if updatedStream.StreamID != streamID {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Stream ID mismatch",
		})
		return
	}

	// 기존 스트림 찾기
	var existingStream *Stream
	for i, stream := range appConfig.StreamList {
		if stream.StreamID == streamID {
			existingStream = &appConfig.StreamList[i]
			break
		}
	}

	if existingStream == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Stream not found",
		})
		return
	}

	// recording은 제외하고 업데이트 (recording은 별도 API로 처리)
	updatedStream.Recording = existingStream.Recording

	result := streamEditReq(updatedStream, METHOD_EDIT)
	if !result {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to update stream",
			"error":   "Failed to update stream",
		})
		return
	}

	// 스트림 찾아서 업데이트
	for i, stream := range appConfig.StreamList {
		if stream.StreamID == streamID {
			appConfig.StreamList[i] = updatedStream
			if err := saveConfig("config.json", appConfig); err != nil {
				log.Printf("Failed to save config: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Failed to update stream",
					"error":   err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"message": "Stream updated successfully",
				"stream":  updatedStream,
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{
		"message": "Stream not found",
	})
}

// updateStreamRecording 스트림 recording 설정
func updateStreamRecording(c *gin.Context) {
	streamID := c.Param("streamID")
	var req struct {
		Recording bool `json:"recording"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid request data",
			"error":   err.Error(),
		})
		return
	}

	// 스트림 찾기
	for i, stream := range appConfig.StreamList {
		if stream.StreamID == streamID {
			// recording 상태 업데이트
			appConfig.StreamList[i].Recording = req.Recording

			// MediaServer에 recording 요청
			streamIds := []string{streamID}
			result := recordingReq(streamIds, req.Recording)
			if !result {
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Failed to update recording",
				})
				return
			}

			// config.json 저장
			if err := saveConfig("config.json", appConfig); err != nil {
				log.Printf("Failed to save config: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Failed to save recording setting",
					"error":   err.Error(),
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message": "Recording updated successfully",
				"stream":  appConfig.StreamList[i],
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{
		"message": "Stream not found",
	})
}

// deleteStream 스트림 삭제
func deleteStream(c *gin.Context) {
	streamID := c.Param("streamID")
	stream := Stream{StreamID: streamID}

	result := streamEditReq(stream, METHOD_DELETE)
	if !result {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to delete stream",
			"error":   "Failed to delete stream",
		})
		return
	}

	for i, stream := range appConfig.StreamList {
		if stream.StreamID == streamID {
			// 스트림 삭제
			appConfig.StreamList = append(appConfig.StreamList[:i], appConfig.StreamList[i+1:]...)
			if err := saveConfig("config.json", appConfig); err != nil {
				log.Printf("Failed to save config: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Failed to delete stream",
					"error":   err.Error(),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"message": "Stream deleted successfully",
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{
		"message": "Stream not found",
	})
}

// saveConfig config.json 파일 저장
func saveConfig(filename string, config *Config) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return err
	}

	return nil
}

// handleWebSocketHealth WebSocket을 통한 health 체크 핸들러
func handleWebSocketHealth(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	log.Println("WebSocket client connected for health check")

	// 주기적으로 health 상태 전송 (1초마다)
	ticker := time.NewTicker(1 * time.Second)
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
			uptime := int64(time.Since(serverStartTime).Seconds())
			status := HealthStatus{
				Status:    "ok",
				Message:   "Client server is running",
				Timestamp: time.Now(),
				Uptime:    uptime,
			}

			data, err := json.Marshal(status)
			if err != nil {
				log.Printf("Failed to marshal status: %v", err)
				continue
			}

			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("Failed to write message: %v", err)
				return
			}

		case <-pingTicker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("Failed to send ping: %v", err)
				return
			}
		}
	}
}
