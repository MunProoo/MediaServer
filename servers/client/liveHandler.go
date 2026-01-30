package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// CoreTerminalOption 스트림 옵션
type CoreTerminalOption struct {
	Name     string `json:"name"`
	StreamID string `json:"streamID"`
}

func HTTPAPICoreLiveView(c *gin.Context) {
	// 템플릿 파일 경로
	tmplPath := filepath.Join("templates", "fullscreenmulti.tmpl")
	templ := template.Must(template.ParseFiles(tmplPath))

	// StreamMap 생성
	streams := make(map[string]CoreTerminalOption)
	for _, stream := range appConfig.StreamList {
		streams[stream.StreamID] = CoreTerminalOption{
			Name:     stream.Name,
			StreamID: stream.StreamID,
		}
	}

	// MediaServer URL 생성
	webrtcUrl := fmt.Sprintf("https://%s:%d",
		appConfig.Server.MediaServer.Address,
		appConfig.Server.MediaServer.Port)

	// TURN Server URL 생성
	turnUrl := fmt.Sprintf("turn:%s:%d",
		appConfig.Server.MediaServer.Address,
		appConfig.Server.TurnServer.Port)
	turnUrlTCP := fmt.Sprintf("turn:%s:%d?transport=tcp",
		appConfig.Server.MediaServer.Address,
		appConfig.Server.TurnServer.Port)

	// 템플릿 데이터 준비
	data := map[string]interface{}{
		"port":    fmt.Sprintf(":%d", appConfig.Server.ClientPort),
		"streams": streams,
		"version": time.Now().String(),
		"options": map[string]interface{}{
			"grid":      4,
			"player":    nil,
			"webrtcUrl": webrtcUrl,
			"turnUrl":   []string{turnUrl, turnUrlTCP},
		},
		"page":  "fullscreenmulti",
		"query": c.Request.URL.Query(),
	}

	// 템플릿 렌더링
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := templ.Execute(c.Writer, data); err != nil {
		log.Printf("Failed to execute template: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Failed to render template",
		})
		return
	}
}
