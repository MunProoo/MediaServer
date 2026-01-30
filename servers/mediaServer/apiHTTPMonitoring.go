package main

import (
	"net/http"

	"log"

	"github.com/gin-gonic/gin"
)

// HTTPAPIMonitoringData 모니터링 데이터 API
func HTTPAPIMonitoringData(c *gin.Context) {
	// 히스토리 포함 여부 파라미터
	includeHistory := c.DefaultQuery("history", "false") == "true"

	data := GetMonitoringData(includeHistory)

	c.JSON(http.StatusOK, data)
}

// HTTPAPIMonitoringSystem 시스템 메트릭만 조회
func HTTPAPIMonitoringSystem(c *gin.Context) {
	Monitoring.mutex.RLock()
	defer Monitoring.mutex.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"system":      Monitoring.SystemMetrics,
		"last_update": Monitoring.LastUpdate,
	})
}

// HTTPAPIMonitoringStreams 스트림 메트릭만 조회
func HTTPAPIMonitoringStreams(c *gin.Context) {
	Monitoring.mutex.RLock()
	defer Monitoring.mutex.RUnlock()

	streams := make([]*StreamMetricsST, 0, len(Monitoring.StreamMetrics))
	for _, metric := range Monitoring.StreamMetrics {
		streams = append(streams, metric)
	}

	c.JSON(http.StatusOK, gin.H{
		"streams":     streams,
		"last_update": Monitoring.LastUpdate,
	})
}

// HTTPAPIMonitoringStream 특정 스트림 메트릭 조회
func HTTPAPIMonitoringStream(c *gin.Context) {
	streamID := c.Param("uuid")

	Monitoring.mutex.RLock()
	defer Monitoring.mutex.RUnlock()

	metric, exists := Monitoring.StreamMetrics[streamID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Stream not found",
		})
		return
	}

	c.JSON(http.StatusOK, metric)
}

// HTTPAPIMonitoringAlerts 경고 목록 조회
func HTTPAPIMonitoringAlerts(c *gin.Context) {
	alerts := generateAlerts()

	c.JSON(http.StatusOK, gin.H{
		"alerts":    alerts,
		"count":     len(alerts),
		"timestamp": Monitoring.LastUpdate,
	})
}

// HTTPAPIMonitoringHistory 히스토리 데이터 조회
func HTTPAPIMonitoringHistory(c *gin.Context) {
	historyData.RLock()
	defer historyData.RUnlock()

	history := MonitoringHistory{
		CPUHistory:     append([]HistoryPoint{}, historyData.cpu...),
		MemoryHistory:  append([]HistoryPoint{}, historyData.memory...),
		NetworkHistory: append([]HistoryPoint{}, historyData.network...),
	}

	c.JSON(http.StatusOK, history)
}

// HTTPAPIMonitoringDashboard 모니터링 대시보드 페이지
func HTTPAPIMonitoringDashboard(c *gin.Context) {
	log.Printf("[INFO] [http_api] [HTTPAPIMonitoringDashboard] Dashboard page accessed")

	c.HTML(http.StatusOK, "monitoring.tmpl", gin.H{
		"port":        Storage.ServerHTTPPort(),
		"serverTitle": "MediaServer 모니터링",
		"version":     "1.0.0",
		"page":        "monitoring",
	})
}

// HTTPAPIMonitoringStats 통계 정보 조회 (간단한 요약)
func HTTPAPIMonitoringStats(c *gin.Context) {
	data := GetMonitoringData(false)

	c.JSON(http.StatusOK, gin.H{
		"summary":     data.Summary,
		"alerts":      data.Alerts,
		"last_update": data.LastUpdate,
		"uptime":      Monitoring.SystemMetrics.Uptime,
	})
}
