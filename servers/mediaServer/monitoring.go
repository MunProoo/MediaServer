package main

import (
	"runtime"
	"strings"
	"sync"
	"time"

	"log"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
)

// MonitoringST 모니터링 시스템 구조체
type MonitoringST struct {
	mutex         sync.RWMutex
	SystemMetrics SystemMetricsST
	StreamMetrics map[string]*StreamMetricsST // streamID -> metrics
	StartTime     time.Time
	LastUpdate    time.Time
}

// SystemMetricsST 시스템 전체 메트릭
type SystemMetricsST struct {
	CPUUsagePercent    float64 `json:"cpu_usage_percent"`
	CPUCores           int     `json:"cpu_cores"`
	MemoryUsedMB       uint64  `json:"memory_used_mb"`
	MemoryTotalMB      uint64  `json:"memory_total_mb"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
	NetworkSentMBPS    float64 `json:"network_sent_mbps"`
	NetworkRecvMBPS    float64 `json:"network_recv_mbps"`
	GoRoutines         int     `json:"goroutines"`
	Uptime             string  `json:"uptime"`
	UptimeSeconds      int64   `json:"uptime_seconds"`

	// 디스크 정보
	DiskFreeMB      float64 `json:"disk_free_mb"`
	DiskTotalMB     float64 `json:"disk_total_mb"`
	DiskUsedPercent float64 `json:"disk_used_percent"`
}

// StreamMetricsST 스트림별 메트릭
type StreamMetricsST struct {
	StreamID     string                     `json:"stream_id"`
	StreamName   string                     `json:"stream_name"`
	Channels     map[string]*ChannelMetrics `json:"channels"`
	TotalClients int                        `json:"total_clients"`
	Status       string                     `json:"status"` // "online", "offline"
	LastUpdate   time.Time                  `json:"last_update"`
}

// ChannelMetrics 채널별 메트릭
type ChannelMetrics struct {
	ChannelID string `json:"channel_id"`
	// URL            string    `json:"url"`
	Status         string    `json:"status"` // "online", "offline"
	ClientCount    int       `json:"client_count"`
	IsRecording    bool      `json:"is_recording"`
	RecordingStart time.Time `json:"recording_start,omitempty"`
	UpTime         string    `json:"uptime"`
	LastKeyFrame   time.Time `json:"last_keyframe"`
	BitrateKbps    float64   `json:"bitrate_kbps"`
}

// MonitoringResponse API 응답 구조체
type MonitoringResponse struct {
	System        SystemMetricsST    `json:"system"`
	Streams       []*StreamMetricsST `json:"streams"`
	Summary       MonitoringSummary  `json:"summary"`
	LastUpdate    time.Time          `json:"last_update"`
	UpdatedFields []string           `json:"updated_fields,omitempty"`
	Alerts        []MonitoringAlert  `json:"alerts,omitempty"`
	History       *MonitoringHistory `json:"history,omitempty"`
}

// MonitoringSummary 요약 정보
type MonitoringSummary struct {
	TotalStreams      int `json:"total_streams"`
	OnlineStreams     int `json:"online_streams"`
	OfflineStreams    int `json:"offline_streams"`
	TotalChannels     int `json:"total_channels"`
	TotalClients      int `json:"total_clients"`
	RecordingChannels int `json:"recording_channels"`
}

// MonitoringAlert 경고 정보
type MonitoringAlert struct {
	Level     string    `json:"level"` // "info", "warning", "critical"
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"` // 경고 발생 위치
}

// MonitoringHistory 히스토리 데이터 (최근 1시간)
type MonitoringHistory struct {
	CPUHistory     []HistoryPoint `json:"cpu_history"`
	MemoryHistory  []HistoryPoint `json:"memory_history"`
	NetworkHistory []HistoryPoint `json:"network_history"`
}

// HistoryPoint 히스토리 데이터 포인트
type HistoryPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

var (
	Monitoring       *MonitoringST
	monitoringTicker *time.Ticker
	historyData      struct {
		sync.RWMutex
		cpu     []HistoryPoint
		memory  []HistoryPoint
		network []HistoryPoint
		maxSize int
	}
	// 네트워크 측정을 위한 이전 값 저장
	lastNetworkStats struct {
		sync.RWMutex
		bytesSent uint64
		bytesRecv uint64
		timestamp time.Time
	}
)

// InitMonitoring 모니터링 시스템 초기화
func InitMonitoring() {
	Monitoring = &MonitoringST{
		SystemMetrics: SystemMetricsST{},
		StreamMetrics: make(map[string]*StreamMetricsST),
		StartTime:     time.Now(),
		LastUpdate:    time.Now(),
	}

	// 히스토리 데이터 초기화 (최대 60개 = 1시간치, 1분마다 업데이트)
	historyData.maxSize = 60
	historyData.cpu = make([]HistoryPoint, 0, historyData.maxSize)
	historyData.memory = make([]HistoryPoint, 0, historyData.maxSize)
	historyData.network = make([]HistoryPoint, 0, historyData.maxSize)

	log.Printf("[INFO] [monitoring] [InitMonitoring] Monitoring system initialized")

	// 주기적으로 메트릭 수집 (5초마다)
	go StartMonitoringLoop()
}

// StartMonitoringLoop 모니터링 루프 시작
func StartMonitoringLoop() {
	monitoringTicker = time.NewTicker(5 * time.Second)
	defer monitoringTicker.Stop()

	historyTicker := time.NewTicker(1 * time.Minute)
	defer historyTicker.Stop()

	for {
		select {
		case <-monitoringTicker.C:
			CollectMetrics()
		case <-historyTicker.C:
			SaveHistoryPoint()
		}
	}
}

// CollectMetrics 모든 메트릭 수집
func CollectMetrics() {
	Monitoring.mutex.Lock()
	defer Monitoring.mutex.Unlock()

	// 시스템 메트릭 수집
	collectSystemMetrics()

	// 스트림 메트릭 수집
	collectStreamMetrics()

	Monitoring.LastUpdate = time.Now()
}

// collectSystemMetrics 시스템 메트릭 수집
func collectSystemMetrics() {
	// CPU 사용률
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercent) > 0 {
		Monitoring.SystemMetrics.CPUUsagePercent = cpuPercent[0]
	}

	// CPU 코어 수
	cpuCount, err := cpu.Counts(true)
	if err == nil {
		Monitoring.SystemMetrics.CPUCores = cpuCount
	}

	// 메모리 사용량
	vmem, err := mem.VirtualMemory()
	if err == nil {
		Monitoring.SystemMetrics.MemoryUsedMB = vmem.Used / 1024 / 1024
		Monitoring.SystemMetrics.MemoryTotalMB = vmem.Total / 1024 / 1024
		Monitoring.SystemMetrics.MemoryUsagePercent = vmem.UsedPercent
	}

	// 네트워크 통계 (초당 전송량 계산)
	// true로 설정하여 모든 네트워크 인터페이스를 개별적으로 가져옴
	netStats, err := net.IOCounters(true)
	if err == nil && len(netStats) > 0 {
		// 모든 인터페이스의 통계를 합산 (loopback 제외)
		var currentBytesSent uint64 = 0
		var currentBytesRecv uint64 = 0

		for _, stat := range netStats {
			// loopback 인터페이스 제외 (lo, lo0, Loopback 등)
			name := strings.ToLower(stat.Name)
			if strings.Contains(name, "loopback") || name == "lo" || name == "lo0" {
				continue
			}

			// 디버그 로그 (5번에 1번만 출력)
			if Monitoring.SystemMetrics.CPUCores > 0 && int(time.Now().Unix())%5 == 0 {
				log.Printf("[DEBUG] [monitoring] [collectSystemMetrics] [iface=%s] [sent=%d] [recv=%d]", stat.Name, stat.BytesSent, stat.BytesRecv)
			}

			currentBytesSent += stat.BytesSent
			currentBytesRecv += stat.BytesRecv
		}

		currentTime := time.Now()

		lastNetworkStats.RLock()
		lastBytesSent := lastNetworkStats.bytesSent
		lastBytesRecv := lastNetworkStats.bytesRecv
		lastTime := lastNetworkStats.timestamp
		lastNetworkStats.RUnlock()

		// 첫 측정이 아닌 경우에만 계산
		if !lastTime.IsZero() {
			timeDiff := currentTime.Sub(lastTime).Seconds()
			if timeDiff > 0 {
				// 초당 전송량 계산 (MB/s)
				sentDiff := float64(currentBytesSent-lastBytesSent) / timeDiff / 1024 / 1024
				recvDiff := float64(currentBytesRecv-lastBytesRecv) / timeDiff / 1024 / 1024

				// 임시: Windows에서 값이 반대로 나오는 것 같아서 swap
				// gopsutil의 BytesSent/BytesRecv 정의가 다를 수 있음
				Monitoring.SystemMetrics.NetworkSentMBPS = recvDiff // 수신 값을 송신으로
				Monitoring.SystemMetrics.NetworkRecvMBPS = sentDiff // 송신 값을 수신으로
			}
		}

		// 현재 값을 저장
		lastNetworkStats.Lock()
		lastNetworkStats.bytesSent = currentBytesSent
		lastNetworkStats.bytesRecv = currentBytesRecv
		lastNetworkStats.timestamp = currentTime
		lastNetworkStats.Unlock()
	}

	// Go 루틴 수
	Monitoring.SystemMetrics.GoRoutines = runtime.NumGoroutine()

	// 업타임
	uptime := time.Since(Monitoring.StartTime)
	Monitoring.SystemMetrics.UptimeSeconds = int64(uptime.Seconds())
	Monitoring.SystemMetrics.Uptime = formatUptime(uptime)

	// 디스크 정보
	freeSpaceGB, totalSpaceGB, err := Storage.getFreeDiskSpace()
	if err == nil {
		Monitoring.SystemMetrics.DiskFreeMB = freeSpaceGB * 1024
		Monitoring.SystemMetrics.DiskTotalMB = totalSpaceGB * 1024
		usedSpaceGB := totalSpaceGB - freeSpaceGB
		if totalSpaceGB > 0 {
			Monitoring.SystemMetrics.DiskUsedPercent = (usedSpaceGB / totalSpaceGB) * 100
		}
	}
}

// collectStreamMetrics 스트림 메트릭 수집
func collectStreamMetrics() {
	Storage.mutex.RLock()
	defer Storage.mutex.RUnlock()

	// 기존 메트릭 초기화
	Monitoring.StreamMetrics = make(map[string]*StreamMetricsST)

	for streamID, stream := range Storage.Streams {
		streamMetric := &StreamMetricsST{
			StreamID:     streamID,
			StreamName:   stream.Name,
			Channels:     make(map[string]*ChannelMetrics),
			TotalClients: 0,
			Status:       "offline",
			LastUpdate:   time.Now(),
		}

		for channelID, channel := range stream.Channels {
			channelMetric := &ChannelMetrics{
				ChannelID: channelID,
				// URL:         channel.URL,
				ClientCount: len(channel.clients),
			}

			// 상태 확인
			if channel.Status == ONLINE {
				channelMetric.Status = "online"
				streamMetric.Status = "online"
				channelMetric.UpTime = formatUptime(time.Since(channel.ack))
			} else {
				channelMetric.Status = "offline"
			}

			// 녹화 상태
			channelMetric.IsRecording = channel.OnRecording
			if channel.Recording != nil && channel.OnRecording {
				channelMetric.RecordingStart = channel.Recording.StartTime
			}

			// 클라이언트 수
			streamMetric.TotalClients += channelMetric.ClientCount

			streamMetric.Channels[channelID] = channelMetric
		}

		Monitoring.StreamMetrics[streamID] = streamMetric
	}
}

// SaveHistoryPoint 히스토리 데이터 포인트 저장
func SaveHistoryPoint() {
	historyData.Lock()
	defer historyData.Unlock()

	now := time.Now()

	// CPU 히스토리
	historyData.cpu = append(historyData.cpu, HistoryPoint{
		Timestamp: now,
		Value:     Monitoring.SystemMetrics.CPUUsagePercent,
	})
	if len(historyData.cpu) > historyData.maxSize {
		historyData.cpu = historyData.cpu[1:]
	}

	// 메모리 히스토리
	historyData.memory = append(historyData.memory, HistoryPoint{
		Timestamp: now,
		Value:     Monitoring.SystemMetrics.MemoryUsagePercent,
	})
	if len(historyData.memory) > historyData.maxSize {
		historyData.memory = historyData.memory[1:]
	}

	// 네트워크 히스토리 (송신+수신 합산)
	networkTotal := Monitoring.SystemMetrics.NetworkSentMBPS + Monitoring.SystemMetrics.NetworkRecvMBPS
	historyData.network = append(historyData.network, HistoryPoint{
		Timestamp: now,
		Value:     networkTotal,
	})
	if len(historyData.network) > historyData.maxSize {
		historyData.network = historyData.network[1:]
	}
}

// GetMonitoringData 모니터링 데이터 조회
func GetMonitoringData(includeHistory bool) MonitoringResponse {
	Monitoring.mutex.RLock()
	defer Monitoring.mutex.RUnlock()

	// 스트림 메트릭을 배열로 변환
	streams := make([]*StreamMetricsST, 0, len(Monitoring.StreamMetrics))
	for _, metric := range Monitoring.StreamMetrics {
		streams = append(streams, metric)
	}

	// 요약 정보 생성
	summary := MonitoringSummary{
		TotalStreams: len(Storage.Streams),
	}

	for _, metric := range Monitoring.StreamMetrics {
		if metric.Status == "online" {
			summary.OnlineStreams++
		} else {
			summary.OfflineStreams++
		}
		summary.TotalChannels += len(metric.Channels)
		summary.TotalClients += metric.TotalClients

		for _, ch := range metric.Channels {
			if ch.IsRecording {
				summary.RecordingChannels++
			}
		}
	}

	response := MonitoringResponse{
		System:     Monitoring.SystemMetrics,
		Streams:    streams,
		Summary:    summary,
		LastUpdate: Monitoring.LastUpdate,
		Alerts:     generateAlerts(),
	}

	// 히스토리 데이터 추가 (선택적)
	if includeHistory {
		historyData.RLock()
		response.History = &MonitoringHistory{
			CPUHistory:     append([]HistoryPoint{}, historyData.cpu...),
			MemoryHistory:  append([]HistoryPoint{}, historyData.memory...),
			NetworkHistory: append([]HistoryPoint{}, historyData.network...),
		}
		historyData.RUnlock()
	}

	return response
}

// generateAlerts 경고 생성
func generateAlerts() []MonitoringAlert {
	alerts := make([]MonitoringAlert, 0)
	now := time.Now()

	// CPU 사용률 경고
	if Monitoring.SystemMetrics.CPUUsagePercent > 90 {
		alerts = append(alerts, MonitoringAlert{
			Level:     "critical",
			Message:   "CPU 사용률이 90%를 초과했습니다",
			Timestamp: now,
			Source:    "system.cpu",
		})
	} else if Monitoring.SystemMetrics.CPUUsagePercent > 75 {
		alerts = append(alerts, MonitoringAlert{
			Level:     "warning",
			Message:   "CPU 사용률이 75%를 초과했습니다",
			Timestamp: now,
			Source:    "system.cpu",
		})
	}

	// 메모리 사용률 경고
	if Monitoring.SystemMetrics.MemoryUsagePercent > 90 {
		alerts = append(alerts, MonitoringAlert{
			Level:     "critical",
			Message:   "메모리 사용률이 90%를 초과했습니다",
			Timestamp: now,
			Source:    "system.memory",
		})
	} else if Monitoring.SystemMetrics.MemoryUsagePercent > 80 {
		alerts = append(alerts, MonitoringAlert{
			Level:     "warning",
			Message:   "메모리 사용률이 80%를 초과했습니다",
			Timestamp: now,
			Source:    "system.memory",
		})
	}

	// 디스크 사용률 경고
	if Monitoring.SystemMetrics.DiskUsedPercent > 90 {
		alerts = append(alerts, MonitoringAlert{
			Level:     "critical",
			Message:   "디스크 사용률이 90%를 초과했습니다",
			Timestamp: now,
			Source:    "system.disk",
		})
	} else if Monitoring.SystemMetrics.DiskUsedPercent > 80 {
		alerts = append(alerts, MonitoringAlert{
			Level:     "warning",
			Message:   "디스크 사용률이 80%를 초과했습니다",
			Timestamp: now,
			Source:    "system.disk",
		})
	}

	// GoRoutines 수 경고
	if Monitoring.SystemMetrics.GoRoutines > 10000 {
		alerts = append(alerts, MonitoringAlert{
			Level:     "critical",
			Message:   "GoRoutines 수가 10,000개를 초과했습니다 (메모리 누수 가능성)",
			Timestamp: now,
			Source:    "system.goroutines",
		})
	} else if Monitoring.SystemMetrics.GoRoutines > 5000 {
		alerts = append(alerts, MonitoringAlert{
			Level:     "warning",
			Message:   "GoRoutines 수가 5,000개를 초과했습니다",
			Timestamp: now,
			Source:    "system.goroutines",
		})
	}

	return alerts
}

// formatUptime 업타임을 읽기 쉬운 형식으로 변환
func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if days > 0 {
		return formatKorean("%d일 %d시간 %d분", days, hours, minutes)
	} else if hours > 0 {
		return formatKorean("%d시간 %d분 %d초", hours, minutes, seconds)
	} else if minutes > 0 {
		return formatKorean("%d분 %d초", minutes, seconds)
	}
	return formatKorean("%d초", seconds)
}

// formatKorean 한글 형식 포맷팅 헬퍼
func formatKorean(format string, args ...interface{}) string {
	switch len(args) {
	case 1:
		return formatString(format, args[0])
	case 2:
		return formatString2(format, args[0], args[1])
	case 3:
		return formatString3(format, args[0], args[1], args[2])
	default:
		return format
	}
}

func formatString(format string, a interface{}) string {
	result := ""
	switch format {
	case "%d초":
		result = formatInt(a) + "초"
	}
	return result
}

func formatString2(format string, a, b interface{}) string {
	result := ""
	switch format {
	case "%d분 %d초":
		result = formatInt(a) + "분 " + formatInt(b) + "초"
	}
	return result
}

func formatString3(format string, a, b, c interface{}) string {
	result := ""
	switch format {
	case "%d일 %d시간 %d분":
		result = formatInt(a) + "일 " + formatInt(b) + "시간 " + formatInt(c) + "분"
	case "%d시간 %d분 %d초":
		result = formatInt(a) + "시간 " + formatInt(b) + "분 " + formatInt(c) + "초"
	}
	return result
}

func formatInt(v interface{}) string {
	switch val := v.(type) {
	case int:
		return intToString(val)
	case int64:
		return intToString(int(val))
	default:
		return "0"
	}
}

func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	if negative {
		result = "-" + result
	}
	return result
}
