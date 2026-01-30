package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"log"

	"github.com/gin-gonic/gin"
)

// 녹화 시작 API
func HTTPAPIServerRecordingStart(c *gin.Context) {
	var err error

	streamID := c.Param("uuid")
	channelID := c.Param("channel")

	defer func() {
		if err != nil {
			log.Printf("[ERROR] [http_recording] [HTTPAPIServerRecordingStart] error=%v", err)
		}
	}()

	// 권한 확인 -> 놓을만한 외부 backend는 alpeta인데, alpeta가 클라이언트 입장이니까 애매하네
	// TODO. 굳이 없어도 될 듯하지만 일단 구색
	if !RemoteAuthorization("recording", streamID, channelID, c.Query("token"), c.ClientIP()) {
		log.Printf("[ERROR] [http_recording] [HTTPAPIServerRecordingStart] error=%s", ErrorStreamUnauthorized.Error())
		c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamUnauthorized.Error()})
	}

	// 1. Config 변경
	Storage.StreamChannelRecordingEnabled(streamID, channelID, true)

	// 스트림 채널 존재 확인
	if !Storage.StreamChannelExist(streamID, channelID) {
		log.Printf("[ERROR] [http_recording] [HTTPAPIServerRecordingStart] error=%s", ErrorStreamNotFound.Error())
		c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		return
	}

	// 녹화 시작 응답

	// 스트림 채널 실행
	Storage.StreamChannelRun(streamID, channelID)
	msgResponse := "await recording started"

	// 녹화 시작
	err = Storage.StartRecording(streamID, channelID)
	if err != nil {
		log.Printf("[ERROR] [http_recording] [HTTPAPIServerRecordingStart] error=%s", err.Error())
		msgResponse = "recording failed"
	}
	c.Writer.Write([]byte(msgResponse))
}

// 녹화 중지 API
func HTTPAPIServerRecordingStop(c *gin.Context) {
	streamID := c.Param("uuid")
	channelID := c.Param("channel")

	// 1. Config 변경
	Storage.StreamChannelRecordingEnabled(streamID, channelID, false)

	// 2. 스트림 녹화 종료
	err := Storage.StopRecording(streamID, channelID)
	if err != nil {
		c.JSON(500, Message{Status: 0, Payload: err.Error()})
		return
	}

	c.JSON(200, Message{Status: 1, Payload: "Recording stopped"})
}

// 녹화 파일 목록 API
// func HTTPAPIServerRecordingList(c *gin.Context) {
// 	streamID := c.Param("uuid")
// 	channelID := c.Param("channel")

// 	recordings, err := Storage.GetRecordings(streamID, channelID)
// 	if err != nil {
// 		c.JSON(500, Message{Status: 0, Payload: err.Error()})
// 		return
// 	}

// 	c.JSON(200, Message{Status: 1, Payload: recordings})
// }

func HTTPAPIServerRecordingStreaming(c *gin.Context) {
	streamID := c.Query("streamID")
	channelID := c.Query("channel")
	findTime := c.Query("findTime") // "2025-09-17 093026"
	dur := c.Query("duration")
	var duration int

	if streamID == "" || channelID == "" || findTime == "" {
		c.IndentedJSON(400, Message{Status: 0, Payload: "Missing required parameter."})
		return
	}

	duration, err := strconv.Atoi(dur)
	if err != nil {
		duration = 6
	}

	// 녹화 파일을 HLS로 스트리밍
	// 녹화 재생 준비
	m3u8Path, err := Storage.VideoServePrepare(streamID, channelID, findTime, duration)
	if err != nil {
		log.Printf("[ERROR] [http_server] [HTTPAPIServerRecordingPlay] error=%s", err.Error())
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		return
	}

	// 상대 경로로 변환
	// relativePath := strings.TrimPrefix(playlistPath, "recordings/")
	// m3u8 파일을 직접 서빙
	c.File(m3u8Path)

	go func() {
		time.Sleep(1 * time.Second)
		os.Remove(m3u8Path)
	}()
}

// 녹화 HLS 파일 서빙 API --> 안씀
func HTTPAPIServerRecordingPlay(c *gin.Context) {
	streamID := c.Param("uuid")
	channelID := c.Param("channel")

	path, err := Storage.VideoServePrepare(streamID, channelID, c.Query("time"), 300)
	if err != nil {
		c.JSON(500, Message{Status: 0, Payload: err.Error()})
		return
	}

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.File(path)
}

// 암호화 키 제공 API
func HTTPAPIServerRecordingKey(c *gin.Context) {
	streamID := c.Param("uuid")
	channelID := c.Param("channel")

	// 권한 확인
	if !RemoteAuthorization("recording", streamID, channelID, c.Query("token"), c.ClientIP()) {
		log.Printf("[ERROR] [http_server] [HTTPAPIServerRecordingKey] error=%s", ErrorStreamUnauthorized.Error())
		c.AbortWithStatus(403)
		return
	}

	// 키 파일 경로
	keyPath := filepath.Join("keys", fmt.Sprintf("%s_%s.key", streamID, channelID))

	// 키 파일 존재 확인
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		log.Printf("[ERROR] [http_server] [HTTPAPIServerRecordingKey] secure key is not exist.")
		c.AbortWithStatus(404)
		return
	}

	// 키 파일 읽기
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		log.Printf("[ERROR] [http_server] [HTTPAPIServerRecordingKey] error=%s", err.Error())
		c.AbortWithStatus(500)
		return
	}

	// 키 제공
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Cache-Control", "no-cache")
	c.Data(200, "application/octet-stream", keyData)
}

// m3u8 파일 직접 서빙 API
func HTTPAPIServerRecordingM3U8File(c *gin.Context) {
	streamID := c.Query("streamID")
	channelID := c.Query("channel")
	m3u8File := c.Query("m3u8File")

	if streamID == "" || channelID == "" || m3u8File == "" {
		c.IndentedJSON(400, Message{Status: 0, Payload: "Missing required parameter: uuid, channel, m3u8File"})
		return
	}

	// m3u8File 이름에서 날짜 추출 (YYYYMMDD_HHMMSS.m3u8 형식)
	// 파일명에서 .m3u8 제거하고 날짜 부분만 추출
	fileName := m3u8File
	if len(fileName) >= 8 {
		dateStr := fileName[:8] // YYYYMMDD
		// YYYYMMDD -> YYYY-MM-DD 형식으로 변환
		if len(dateStr) == 8 {
			date := dateStr[:4] + "-" + dateStr[4:6] + "-" + dateStr[6:8]

			// 파일 경로 구성: recordings/{date}/{streamID}/{channelID}/{m3u8File}
			root := Storage.Server.Maintenance.RetentionRoot
			m3u8Path := filepath.Join(root, date, streamID, channelID, m3u8File)

			// 파일 존재 확인
			if _, err := os.Stat(m3u8Path); os.IsNotExist(err) {
				log.Printf("[ERROR] [http_server] [HTTPAPIServerRecordingM3U8File] m3u8 file not found: path=%s", m3u8Path)
				c.IndentedJSON(404, Message{Status: 0, Payload: "M3U8 file not found"})
				return
			}

			// m3u8 파일 서빙
			c.Header("Content-Type", "application/vnd.apple.mpegurl")
			c.Header("Cache-Control", "no-cache")
			c.File(m3u8Path)
			return
		}
	}

	c.IndentedJSON(400, Message{Status: 0, Payload: "Invalid m3u8File name format. Expected: YYYYMMDD_HHMMSS.m3u8"})
}

// MultiRecordingRequest 다중 녹화 요청 구조체
type MultiRecordingRequest struct {
	StreamIds []string `json:"streamIds" binding:"required"`
}

// MultiRecordingResponse 다중 녹화 응답 구조체
type MultiRecordingResponse struct {
	StreamID string `json:"streamId"`
	Success  bool   `json:"success"`
	Message  string `json:"message"`
}

// HTTPAPIServerRecordingMultiStart 다중 스트림 녹화 시작 API
func HTTPAPIServerRecordingMultiStart(c *gin.Context) {
	var request MultiRecordingRequest

	// JSON 파싱
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Printf("[ERROR] [http_recording] [HTTPAPIServerRecordingMultiStart] error=%v", err)
		c.JSON(400, Message{Status: 0, Payload: "Invalid request format: " + err.Error()})
		return
	}

	// streamIds가 비어있는 경우
	if len(request.StreamIds) == 0 {
		c.JSON(400, Message{Status: 0, Payload: "streamIds array is empty"})
		return
	}

	// 결과를 담을 슬라이스
	results := make([]MultiRecordingResponse, len(request.StreamIds))

	// 동시 실행을 위한 WaitGroup
	var wg sync.WaitGroup

	// 각 스트림에 대해 녹화 시작
	for i, streamId := range request.StreamIds {
		wg.Add(1)

		go func(index int, streamID string) {
			defer wg.Done()

			channelID := "0"

			response := MultiRecordingResponse{
				StreamID: streamID,
				Success:  false,
			}

			// 권한 확인
			if !RemoteAuthorization("recording", streamID, channelID, c.Query("token"), c.ClientIP()) {
				response.Message = "Unauthorized"
				results[index] = response
				log.Printf("[ERROR] [http_recording] [HTTPAPIServerRecordingMultiStart] stream=%s error=Unauthorized", streamID)
				return
			}

			// 스트림 채널 존재 확인
			if !Storage.StreamChannelExist(streamID, channelID) {
				response.Message = "Stream not found"
				results[index] = response
				log.Printf("[ERROR] [http_recording] [HTTPAPIServerRecordingMultiStart] stream=%s error=Stream not found", streamID)
				return
			}

			// Config 변경
			Storage.StreamChannelRecordingEnabled(streamID, channelID, true)

			// 스트림 채널 실행
			Storage.StreamChannelRun(streamID, channelID)

			// 녹화 시작
			err := Storage.StartRecording(streamID, channelID)
			if err != nil {
				response.Message = "Recording failed: " + err.Error()
				results[index] = response
				log.Printf("[ERROR] [http_recording] [HTTPAPIServerRecordingMultiStart] stream=%s error=%v", streamID, err)
				return
			}

			response.Success = true
			response.Message = "Recording started"
			results[index] = response

		}(i, streamId)
	}

	// 모든 작업이 완료될 때까지 대기
	wg.Wait()

	// 성공/실패 카운트
	successCount := 0
	failCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failCount++
		}
	}

	log.Printf("[INFO] [http_recording] [HTTPAPIServerRecordingMultiStart] total=%d success=%d failed=%d", len(results), successCount, failCount)

	c.JSON(200, gin.H{
		"status": 1,
		"payload": gin.H{
			"total":   len(results),
			"success": successCount,
			"failed":  failCount,
			"results": results,
		},
	})
}

// HTTPAPIServerRecordingMultiStop 다중 스트림 녹화 중지 API
func HTTPAPIServerRecordingMultiStop(c *gin.Context) {
	var request MultiRecordingRequest

	// JSON 파싱
	if err := c.ShouldBindJSON(&request); err != nil {
		log.Printf("[ERROR] [http_recording] [HTTPAPIServerRecordingMultiStop] error=%v", err)
		c.JSON(400, Message{Status: 0, Payload: "Invalid request format: " + err.Error()})
		return
	}

	// streamIds가 비어있는 경우
	if len(request.StreamIds) == 0 {
		c.JSON(400, Message{Status: 0, Payload: "streamIds array is empty"})
		return
	}

	// 결과를 담을 슬라이스
	results := make([]MultiRecordingResponse, len(request.StreamIds))

	// 동시 실행을 위한 WaitGroup
	var wg sync.WaitGroup

	// 각 스트림에 대해 녹화 중지
	for i, streamId := range request.StreamIds {
		wg.Add(1)

		go func(index int, streamID string) {
			defer wg.Done()

			channelID := "0"

			response := MultiRecordingResponse{
				StreamID: streamID,
				Success:  false,
			}

			// Config 변경
			Storage.StreamChannelRecordingEnabled(streamID, channelID, false)

			// 녹화 중지
			err := Storage.StopRecording(streamID, channelID)
			if err != nil {
				response.Message = "Stop failed: " + err.Error()
				results[index] = response
				log.Printf("[ERROR] [http_recording] [HTTPAPIServerRecordingMultiStop] error=%v", err)
				return
			}

			response.Success = true
			response.Message = "Recording stopped"
			results[index] = response

		}(i, streamId)
	}

	// 모든 작업이 완료될 때까지 대기
	wg.Wait()

	// 성공/실패 카운트
	successCount := 0
	failCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failCount++
		}
	}

	log.Printf("[INFO] [http_recording] [HTTPAPIServerRecordingMultiStop] total=%d success=%d failed=%d", len(results), successCount, failCount)

	c.JSON(200, gin.H{
		"status": 1,
		"payload": gin.H{
			"total":   len(results),
			"success": successCount,
			"failed":  failCount,
			"results": results,
		},
	})
}

// HTTPAPIServerRecordingListByDate 날짜별 녹화 파일 목록 조회 API
func HTTPAPIServerRecordingListByDate(c *gin.Context) {
	streamID := c.Query("streamID")
	date := c.Query("date") // 형식: 2025-01-07

	// 파라미터 검증
	if streamID == "" {
		c.JSON(400, Message{Status: 0, Payload: "streamId is required"})
		return
	}

	if date == "" {
		c.JSON(400, Message{Status: 0, Payload: "date parameter is required (format: YYYY-MM-DD)"})
		return
	}

	// 날짜 형식 검증
	_, err := time.Parse("2006-01-02", date)
	if err != nil {
		c.JSON(400, Message{Status: 0, Payload: "Invalid date format. Use YYYY-MM-DD"})
		return
	}

	// 녹화 파일 목록 조회
	recordings, err := Storage.GetRecordingListByDate(streamID, date)
	if err != nil {
		log.Printf("[ERROR] [http_recording] [HTTPAPIServerRecordingListByDate] stream=%s date=%s error=%v", streamID, date, err)
		c.JSON(500, Message{Status: 0, Payload: err.Error()})
		return
	}

	log.Printf("[INFO] [http_recording] [HTTPAPIServerRecordingListByDate] retrieved recording list successfully: stream=%s date=%s count=%d", streamID, date, len(recordings))

	c.JSON(200, gin.H{
		"status": 1,
		"payload": gin.H{
			"streamId":   streamID,
			"date":       date,
			"count":      len(recordings),
			"recordings": recordings,
		},
	})
}
