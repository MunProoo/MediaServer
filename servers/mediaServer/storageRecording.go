package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"log"
)

// 녹화 시작 (HLS 방식)
func (obj *StorageST) StartRecording(streamID, channelID string) error {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()

	// 1. 녹화 키 생성
	recordingKey := fmt.Sprintf("%s_%s", streamID, channelID)

	// 2. 기존 녹화가 이미 진행 중이면 그냥 반환
	if existingRecording, exists := obj.Recordings[recordingKey]; exists {
		if existingRecording.Status == RecordingOn {
			log.Printf("[WARN] [recording] [StartRecording] already recording: stream=%s", existingRecording.StreamName)
			return nil
		}
	}

	streamName := obj.Streams[streamID].Name

	now := time.Now()
	creation_time := now.Format("2006-01-02 15:04:05Z")

	// 녹화 세션 ID 생성 (시작 시간 기반)
	sessionID := now.Format("20060102_150405")

	// 녹화 디렉토리 생성
	root := obj.Server.Maintenance.RetentionRoot
	saveDir := filepath.Join(root, creation_time[:10], streamID, channelID)
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return err
	}

	// 로그 파일
	ffmpegLog := filepath.Join(saveDir, "ffmpeg.log")
	logFileHandle, err := os.OpenFile(ffmpegLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	// RTSP URL 생성
	rtspURL := fmt.Sprintf("rtsp://localhost%s/%s/%s",
		obj.Server.RTSPPort, streamID, channelID)

	// 암호화 키 생성 또는 기존 키 사용
	keyinfoPath, err := obj.getOrCreateEncryptionKey(streamID, channelID, streamName)
	if err != nil {
		logFileHandle.Close()
		return fmt.Errorf("암호화 키 생성 실패: %v", err)
	}

	// 녹화 시작 마커 추가
	startMarker := fmt.Sprintf(`=============================================== 녹화 시작 ===============================================
	세션ID: %s
	시작시간: %s
	RTSP URL: %s
=====================================================================================================
`, sessionID, now.Format("2006-01-02 15:04:05"), rtspURL)
	logFileHandle.WriteString(startMarker)

	// HLS 파일 경로 설정
	playlistPath := filepath.Join(saveDir, fmt.Sprintf("%s.m3u8", sessionID))
	segmentPrefix := filepath.Join(saveDir, fmt.Sprintf("%s_%%Y%%m%%d_%%H%%M%%S.ts",
		streamName))

	ffmpegName := "ffmpeg"
	if runtime.GOOS == "windows" {
		ffmpegName = "ffmpeg.exe"
	}
	tempFFmpegPath := filepath.Join(obj.Server.FFMPEGPath, ffmpegName)

	// HLS FFmpeg 명령어 생성
	// -map 0:v:0: 비디오 스트림 명시적 매핑
	// -map 0:a:0?: 오디오 스트림 매핑 (있으면 포함, 없으면 무시)
	// pcm_mulaw 같은 비표준 코덱은 AAC로 변환하여 브라우저 호환성 확보
	command := exec.Command(tempFFmpegPath,
		"-rtsp_transport", "tcp",
		"-fflags", "+genpts+discardcorrupt",
		"-i", rtspURL,
		"-map", "0:v:0", // 비디오 스트림 매핑
		"-map", "0:a:0?", // 오디오 스트림 매핑 (있으면 포함)
		"-c:v", "copy", // 비디오 복사
		"-c:a", "aac", // 오디오를 AAC로 변환 (브라우저 호환성)
		"-b:a", "128k", // 오디오 비트레이트
		"-ar", "48000", // 오디오 샘플레이트 (표준)
		"-ac", "2", // 스테레오로 변환 (모노인 경우)
		"-f", "hls",
		"-hls_time", "10", // 10초 단위 세그먼트
		"-hls_list_size", "0", // playlist에 모든 세그먼트 기록
		"-hls_key_info_file", keyinfoPath, // 암호화 키 정보 파일
		"-strftime", "1",
		"-hls_segment_filename", segmentPrefix,
		// "-hls_flags", "delete_segments", // 재시작 시 이전 세그먼트 삭제
		"-y",
		playlistPath,
	)

	// io.Pipe를 생성하여 stdin에 쓰기 가능하게 연결
	pr, pw := io.Pipe()
	command.Stdin = pr

	// stderr를 파이프로 연결하여 실시간 모니터링
	stderrPipe, err := command.StderrPipe()
	if err != nil {
		logFileHandle.Close()
		return err
	}

	// 녹화 정보 생성
	recording := &RecordingST{
		StreamID:      streamID,
		ChannelID:     channelID,
		StartTime:     now,
		Status:        RecordingOn,
		FFmpegCmd:     command,
		StopSignal:    make(chan bool, 1),
		doneChan:      make(chan bool, 1),
		pw:            pw,
		SessionID:     sessionID,
		PlaylistPath:  playlistPath,
		SegmentDir:    saveDir,
		SegmentPrefix: segmentPrefix,
		StreamName:    streamName,
	}

	// FFmpeg 프로세스 시작
	if err := command.Start(); err != nil {
		logFileHandle.Close()
		return err
	}

	// 녹화 정보 저장
	obj.Recordings[recordingKey] = recording

	// 녹화 모니터링 고루틴 시작
	go obj.monitorRecording(recording, logFileHandle, stderrPipe)

	log.Printf("[INFO] [recording] [StartRecording] Recording started in HLS format.: stream=%s", streamName)

	return nil
}

// 녹화 모니터링 (FFmpeg 출력 감시 + 프로세스 종료 대기 통합)
func (obj *StorageST) monitorRecording(recording *RecordingST, logFileHandle *os.File, stderrPipe io.ReadCloser) {
	// 컨텍스트 생성 (모든 고루틴 종료 제어용)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		if logFileHandle != nil {
			// 녹화 종료 마커 추가
			endTime := time.Now()
			endMarker := fmt.Sprintf(`=============================================== 녹화 종료 ===============================================
			세션ID: %s
			종료시간: %s
			녹화시간: %v
=====================================================================================================
`, recording.SessionID, endTime.Format("2006-01-02 15:04:05"), endTime.Sub(recording.StartTime))
			logFileHandle.WriteString(endMarker)
			logFileHandle.Close()
		}
		if stderrPipe != nil {
			stderrPipe.Close()
		}
		close(recording.doneChan)
		cancel() // 함수 종료 시 모든 고루틴 정리
	}()

	processExited := make(chan error, 1) // cmd 종료 전파 채널
	lastOutputTime := time.Now()         // 마지막 출력 시간
	var outputMutex sync.Mutex

	// 에러 감지 고루틴 (출력이 안되고 있다 -> 녹화가 안되고 있다 -> 에러)
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()

			// 로그 파일에 기록
			if logFileHandle != nil {
				logFileHandle.WriteString(line + "\n")
			}

			// 출력이 있으면 마지막 출력 시간 갱신
			outputMutex.Lock()
			lastOutputTime = time.Now()
			outputMutex.Unlock()
		}
	}()

	// 2. 에러 (타임아웃) 모니터링 (별도 고루틴)
	timeoutDetected := make(chan bool, 1) // 에러 전파 채널
	go func() {
		ticker := time.NewTicker(10 * time.Second) // 10초마다 체크
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				outputMutex.Lock()
				elapsed := time.Since(lastOutputTime)
				outputMutex.Unlock()

				if elapsed > 10*time.Second {
					log.Printf("[WARN] [recording] [monitorRecording] FFmpeg output has timed out.: stream=%s elapsed=%v", recording.StreamName, elapsed)

					select {
					case timeoutDetected <- true:
					default:
					}
					return
				}
			case <-ctx.Done():
				// 컨텍스트 취소 시 고루틴 종료
				return
			}
		}
	}()

	// 3. FFmpeg 프로세스 종료 대기
	go func() {
		err := recording.FFmpegCmd.Wait() // 종료 명령 받을 때까지 대기
		// log.WithFields(logrus.Fields{
		// 	"module": "recording",
		// 	"fn":   "monitorRecording",
		// }).Infof("FFmpeg Wait() 완료, 채널 전송 시도: %s_%s, err: %v", recording.StreamID, recording.ChannelID, err)

		processExited <- err

		// log.WithFields(logrus.Fields{
		// 	"module": "recording",
		// 	"fn":   "monitorRecording",
		// }).Infof("FFmpeg Wait() 채널 전송 완료: %s_%s", recording.StreamID, recording.ChannelID)
	}()

	// 4. 프로세스 종료 대기 및 처리해주는 함수
	obj.handleFFmpegExit(recording, processExited, timeoutDetected)
}

// handleFFmpegExit FFmpeg 프로세스 종료 처리 및 재시작 판단
func (obj *StorageST) handleFFmpegExit(recording *RecordingST, processExited chan error, timeoutDetected chan bool) {
	// 종료 원인 파악
	var exitErr error
	var exitReason string

	// 여기서 blocking 되므로 호출한 monitoring도 return 안됨.
	select {
	case exitErr = <-processExited:
		// 프로세스가 자체 종료됨
		if recording.StoppedByUser {
			exitReason = "user_stopped"
		} else if exitErr != nil {
			exitReason = "process_error"
		} else {
			exitReason = "scheduled_restart" // user_stopped로 통합되었으니 안나와야 정상
		}

	case <-timeoutDetected:
		// 타임아웃으로 강제 종료
		exitReason = "timeout"

		// stdin 파이프 닫기 (Wait() 블록 해제용)
		if recording.pw != nil {
			recording.pw.Close()
		}

		// 2. 프로세스 Kill
		if recording.FFmpegCmd.Process != nil {
			recording.FFmpegCmd.Process.Kill()
			exitErr = <-processExited
		}
	}

	// 로깅
	log.Printf("[INFO] [recording] [handleFFmpegExit] record finished.: reason=%s stream=%s", exitReason, recording.StreamName)

	// 재시작 여부 판단 및 실행
	if recording.Status == RecordingOn {
		obj.decideAndRestart(recording, exitReason)
	}
}

// decideAndRestart 재시작 여부 판단 및 실행
func (obj *StorageST) decideAndRestart(recording *RecordingST, exitReason string) {
	// 종료 원인별 처리
	switch exitReason {
	case "user_stopped":
		// 사용자가 중지 버튼 클릭 → 재시작 안함
		log.Printf("[INFO] [recording] [decideAndRestart] normal recording stop: stream=%s", recording.StreamName)

	case "scheduled_restart":
		// 24시 자동 재시작 등 → 재시작 필요
		log.Printf("[INFO] [recording] [decideAndRestart] auto restart scheduled at 00:00.: stream=%s", recording.StreamName)
		go obj.RestartRecordingStream(recording.StreamID, recording.ChannelID)

	case "process_error", "timeout":
		// 에러로 인한 종료 → 재시작 필요
		recording.Status = RecordingErr
		log.Printf("[WARN] [recording] [decideAndRestart] restart triggered due to abnormal termination.: reason=%s stream=%s", exitReason, recording.StreamName)
		go obj.RestartRecordingStream(recording.StreamID, recording.ChannelID)

	default:
		// 알 수 없는 원인
		log.Printf("[WARN] [recording] [decideAndRestart] restart skipped: Unknown termination cause.: reason=%s stream=%s", exitReason, recording.StreamName)
	}
}

// 녹화 재시작
func (obj *StorageST) RestartRecordingStream(streamID, channelID string) {
	streamName := obj.Streams[streamID].Name
	// 스트림 채널이 여전히 녹화 활성화 상태인지 확인
	channelInfo, err := obj.StreamChannelInfo(streamID, channelID)
	if err != nil {
		log.Printf("[ERROR] [recording] [RestartRecordingStream] failed to find channel: stream=%s error=%v", streamName, err)
		return
	}

	// OnRecording이 false면 재시작 안함
	if !channelInfo.OnRecording {
		log.Printf("[WARN] [recording] [RestartRecordingStream] recording status is off: stream=%s", streamName)
		return
	}

	// 녹화 재시작 진행
	log.Printf("[INFO] [recording] [RestartRecordingStream] recording restart: stream=%s", streamName)

	recordingKey := fmt.Sprintf("%s_%s", streamID, channelID)

	// 기존 녹화 중지 (있다면)
	obj.mutex.RLock()
	_, exists := obj.Recordings[recordingKey]
	obj.mutex.RUnlock()

	if exists {
		log.Printf("[INFO] [recording] [RestartRecordingStream] prev recording stopped.: stream=%s", streamName)

		if err := obj.StopRecording(streamID, channelID); err != nil {
			log.Printf("[ERROR] [recording] [RestartRecordingStream] failed to stop prev recording: stream=%s error=%v", streamName, err)
			return
		}

		time.Sleep(1 * time.Second) // 완전히 정리될 때까지 대기
	}

	// 새로운 녹화 시작
	if err := obj.StartRecording(streamID, channelID); err != nil {
		log.Printf("[ERROR] [recording] [RestartRecordingStream] failed to restart recording: stream=%s error=%v", streamName, err)
	}
}

// 녹화 중지
func (obj *StorageST) StopRecording(streamID, channelID string) error {
	log.Println("FFMPEG 종료 명령이 옴")
	streamName := obj.Streams[streamID].Name

	recordingKey := fmt.Sprintf("%s_%s", streamID, channelID)
	obj.mutex.RLock()
	recording, exist := obj.Recordings[recordingKey]
	obj.mutex.RUnlock()

	if !exist || recording.Status != RecordingOn {
		return nil // 이미 중지됨
	}

	log.Printf("[INFO] [recording] [StopRecording] recording stop req: stream=%s", streamName)

	// 사용자에 의한 정상 종료로 표시
	recording.StoppedByUser = true

	// FFmpeg 프로세스 종료
	if _, err := recording.pw.Write([]byte("q\n")); err != nil {
		log.Fatalf("FFMPEG 종료 명령 전달 실패 : %v", err)
		return err
	}
	recording.pw.Close()

	// ffmpeg 프로세스가 종료될 때까지 대기(비동기로 동작 시 필요에 따라 스킵 가능)
	<-recording.doneChan

	// 녹화 완료 처리
	recording.Status = RecordingOff
	recording.EndTime = time.Now()

	// 녹화 목록에서 삭제
	obj.mutex.Lock()
	delete(obj.Recordings, recordingKey)
	obj.mutex.Unlock()

	return nil
}

// 녹화중인 목록 조회...
func (obj *StorageST) GetRecordings() []RecordingST {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()

	list := make([]RecordingST, 0)
	for _, recording := range obj.Recordings {
		list = append(list, *recording)
	}

	return list
}

// 녹화중인 모든 카메라 녹화 재시작
func (obj *StorageST) AllStreamRestartRecording() {
	log.Println("녹화 재시작")
	for _, recording := range obj.Recordings {
		// 순차적으로 재시작 (시스템 부하 분산)
		go obj.RestartRecordingStream(recording.StreamID, recording.ChannelID)
	}
}

// HLS 세그먼트 정리
func (obj *StorageST) cleanupHLSSegments(recording *RecordingST) {
	if recording.SegmentDir == "" {
		return
	}

	// 세그먼트 디렉토리의 모든 .ts 파일 삭제
	err := filepath.Walk(recording.SegmentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".ts" {
			if err := os.Remove(path); err != nil {
				log.Printf("[ERROR] [recording] [cleanupHLSSegments] 세그먼트 파일 삭제 실패: path=%s error=%v", path, err)
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("[ERROR] [recording] [cleanupHLSSegments] 세그먼트 정리 중 오류: error=%v", err)
	}
}

// 녹화 파일을 HLS로 스트리밍
func (obj *StorageST) VideoServePrepare(streamID, channelID, findTime string, duration int) (string, error) {
	// findTime = "2025-09-17 093026"

	log.Printf("[INFO] [recording] [VideoServePrepare] prepared recording video")

	// 1. 날짜 파싱
	targetDate, err := time.Parse("2006-01-02 150405", findTime)
	if err != nil {
		return "", fmt.Errorf("시간 형식 오류: %v", err)
	}

	// 2. 녹화 경로 구성: ./recordings/날짜/streamID/channelID/
	root := obj.Server.Maintenance.RetentionRoot
	dateStr := targetDate.Format("2006-01-02")
	recordingDir := filepath.Join(root, dateStr, streamID, channelID)

	// 3. 녹화 디렉토리 존재 확인
	if _, err := os.Stat(recordingDir); os.IsNotExist(err) {
		return "", fmt.Errorf("video is not exist: %s", recordingDir)
	}

	// 4. m3u8이 이미 있으면 전달. 없으면 생성
	m3u8Path := filepath.Join(recordingDir, fmt.Sprintf("%s.m3u8", targetDate.Format("20060102_150405")))
	if _, err := os.Stat(m3u8Path); err == nil {
		return m3u8Path, nil
	}

	// 5. 해당 시간대의 세그먼트 찾기
	segments, err := obj.findSegmentsInTimeRange(streamID, channelID, recordingDir, targetDate, time.Duration(duration)*time.Second)
	if err != nil {
		return "", fmt.Errorf("failed to find segments: %v", err)
	}

	if len(segments) == 0 {
		return "", fmt.Errorf("video is not exist: %s", findTime)
	}

	// 6. 임시 m3u8 파일 생성
	// tempPlaylistPath, err := obj.createTempPlaylist(recordingDir, segments, targetDate)
	// if err != nil {
	// 	return "", fmt.Errorf("임시 플레이리스트 생성 실패: %v", err)
	// }
	tempPlayListPath, err := obj.createTempM3U8(recordingDir, segments, targetDate)
	if err != nil {
		return "", fmt.Errorf("faield to create m3u8: %v", err)
	}

	// out.Printi(out.LogArg{"module": "recording", "fn": "VideoServePrepare", "text": "임시 플레이리스트 생성 완료", "path": tempPlayListPath})

	return tempPlayListPath, nil
}

// 시간 범위 내의 세그먼트 찾기
func (obj *StorageST) findSegmentsInTimeRange(streamID, channelID, recordingDir string, targetTime time.Time, duration time.Duration) ([]SegmentInfo, error) {
	var segments []SegmentInfo
	endTime := targetTime.Add(duration)

	// out.Printi(out.LogArg{"module": "recording", "fn": "findSegmentsInTimeRange", "text": "세그먼트 검색 시작", "recordingDir": recordingDir, "targetTime": targetTime.Format("20060102_150405"), "endTime": endTime.Format("20060102_150405")})

	// 1. 타겟 시간 10초 내려서 찾기
	baseTime := targetTime.Add(-10 * time.Second)

	// 2. 검색할 후보 파일 생성 (시작 시간 세그먼트 ~ 종료 시간 세그먼트)
	candidateTimes := obj.findCandidateTimes(baseTime, endTime)

	// 3. 각 후보 파일에 대해 os.Stat으로 존재확인 (O(1))
	for _, candidateTime := range candidateTimes {
		// 파일명 생성 (streamID_channelID_YYYYMMDD_HHMMSS.ts)
		fileName := obj.generateSegmentFileName(streamID, channelID, candidateTime)
		candidateFile := filepath.Join(recordingDir, fileName)

		// 파일 존재 확인
		if info, err := os.Stat(candidateFile); err == nil {
			// 파일 존재함
			segments = append(segments, SegmentInfo{
				FilePath: candidateFile,
				FileName: fileName,
				Time:     candidateTime, // 정렬에서 사용
				Size:     info.Size(),   // 굳이? 재생 길이는 있으면 m3u8 만들 때 좋긴 한데...
			})

			// out.Printi(out.LogArg{"module": "recording", "fn": "findSegmentsInTimeRange", "text": "세그먼트 발견", "fileName": fileName})

			// 바로 break를 해야하나.. 더 돌아야한다.
		}
	}

	// 4. 시간순으로 정렬
	// 굳이 필요한가? 어차피 1부터 증가하는데?
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].Time.Before(segments[j].Time)
	})

	log.Printf("[INFO] [recording] [findSegmentsInTimeRange] success to find segments: count=%d", len(segments))

	return segments, nil
}

// 원하는 녹화 영상의 세그먼트 후보 찾기.
// 10초 단위로 녹화되므로 해당하는 이름의 세그먼트를 찾는다. (150405 라면 150400부터 찾는다.)
func (obj *StorageST) findCandidateTimes(baseTime, endTime time.Time) []time.Time {
	var candidates []time.Time

	// candidates = append(candidates, baseTime)
	currentTime := baseTime

	// 파일이름이 언제일 지 모르므로 1~9초까지 확인 필요
	for currentTime.Before(endTime) || currentTime.Equal(endTime) {
		candidates = append(candidates, currentTime)
		currentTime = currentTime.Add(1 * time.Second)
	}

	return candidates
}

// 세그먼트 파일명 만들어서 return
// 파일명 형식: streamID_channelID_YYYYMMDD_HHMMSS.ts
func (obj *StorageST) generateSegmentFileName(streamID, channelID string, segmentTime time.Time) string {
	streamName := obj.Streams[streamID].Name

	dateStr := segmentTime.Format("20060102")
	timeStr := segmentTime.Format("150405")
	return fmt.Sprintf("%s_%s_%s.ts", streamName, dateStr, timeStr)
}

// 임시 플레이리스트 생성
// 암호화된 이후로는 ffprobe가 실패해서 유기.
func (obj *StorageST) createTempPlaylist(recordingDir string, segments []SegmentInfo, targetTime time.Time) (string, error) {
	// 임시 플레이리스트 파일명 생성
	tempFileName := fmt.Sprintf("%s.m3u8",
		targetTime.Format("20060102_150405"))
	tempPlaylistPath := filepath.Join(recordingDir, tempFileName)

	// m3u8 파일 생성
	file, err := os.Create(tempPlaylistPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// m3u8 헤더 작성
	fmt.Fprintf(file, "#EXTM3U\n")
	fmt.Fprintf(file, "#EXT-X-VERSION:3\n")
	fmt.Fprintf(file, "#EXT-X-TARGETDURATION:12\n")
	fmt.Fprintf(file, "#EXT-X-MEDIA-SEQUENCE:0\n")

	// 세그먼트 정보 작성
	for _, segment := range segments {
		output, err := obj.findVideoMetaData(segment.FilePath)
		if err != nil {
			log.Println(err)
			continue
		}

		fmt.Fprintf(file, "#EXTINF:%s,\n", output.Format.Duration)
		fmt.Fprintf(file, "%s\n", segment.FileName)
	}

	fmt.Fprintf(file, "#EXT-X-ENDLIST\n")

	return tempPlaylistPath, nil
}

// 세그먼트 정보 구조체
type SegmentInfo struct {
	FilePath string
	FileName string
	Time     time.Time
	Size     int64
}

// 암호화된 이후로는 ffprobe가 실패해서 유기.
func (obj *StorageST) findVideoMetaData(videoPath string) (format FFProbeOutput, err error) {
	ffprobeName := "ffprobe"
	if runtime.GOOS == "windows" {
		ffprobeName = "ffprobe.exe"
	}
	tempFFProbePath := filepath.Join(obj.Server.FFMPEGPath, ffprobeName)
	cmd := exec.Command(tempFFProbePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_entries", "format=duration:format_tags=creation_time",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return
	}

	if err = json.Unmarshal(output, &format); err != nil {
		return
	}

	return
}

// 암호화 키 가져오기 또는 생성
func (obj *StorageST) getOrCreateEncryptionKey(streamID, channelID, streamName string) (string, error) {
	// 키 디렉토리 생성
	keyDir := filepath.Join("keys")
	if err := os.MkdirAll(keyDir, 0755); err != nil {
		return "", err
	}

	keyinfoPath := filepath.Join(keyDir, fmt.Sprintf("%s_%s.keyinfo", streamID, channelID))

	// 기존 keyinfo 파일이 있으면 재사용
	if _, err := os.Stat(keyinfoPath); err == nil {
		log.Printf("[INFO] [recording] [getOrCreateEncryptionKey] use prev secure key: stream=%s", streamName)
		return keyinfoPath, nil
	}

	// 새로운 키 생성
	log.Printf("[INFO] [recording] [getOrCreateEncryptionKey] create new secure key: stream=%s", streamName)

	// 1. 랜덤 AES-128 키 생성 (16 bytes)
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}

	// 2. 키 파일 저장
	keyPath := filepath.Join(keyDir, fmt.Sprintf("%s_%s.key", streamID, channelID))
	if err := os.WriteFile(keyPath, key, 0600); err != nil {
		return "", err
	}

	// 3. IV 생성 (16 bytes)
	iv := make([]byte, 16)
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}

	// 4. 키 URL (API 엔드포인트)
	keyURL := fmt.Sprintf("/stream/%s/channel/%s/recording/key", streamID, channelID)

	// 5. keyinfo 파일 생성
	// 형식: 키URL\n키파일경로\nIV(hex)
	keyinfoContent := fmt.Sprintf("%s\n%s\n%x", keyURL, keyPath, iv)
	if err := os.WriteFile(keyinfoPath, []byte(keyinfoContent), 0600); err != nil {
		return "", err
	}

	return keyinfoPath, nil
}

// 녹화 파일 암호화 이후 ------------------------------------------------------------------------------------------------
// 임시 플레이리스트 생성
func (obj *StorageST) createTempM3U8(recordingDir string, segments []SegmentInfo, targetTime time.Time) (string, error) {
	var encryptionKeyLine string // 암호화 관련 정보

	// 1. m3u8 파일들 찾기
	m3u8Files, err := filepath.Glob(filepath.Join(filepath.Dir(segments[0].FilePath), "*.m3u8"))
	if err == nil && len(m3u8Files) > 0 {
		sort.Strings(m3u8Files) // 파일 이름 순 (녹화된 시간 순 정렬)

		// 암호키 추출
		for _, m3u8Path := range m3u8Files {
			keyLine, err := obj.extractEncryptionKey(m3u8Path)
			if err == nil && keyLine != "" {
				encryptionKeyLine = keyLine
				break
			}
		}

		// 2. m3u8 파일에서 세그먼트 관련 정보 추출 (재생시간)
		segmentInfoMap, err := obj.findM3U8ForSegment(m3u8Files, segments)
		if err != nil || segmentInfoMap == nil {
			return "", fmt.Errorf("failed to extract duration")
		}

		// 3. tempM3U8 생성
		tempM3U8Path, err := obj.createM3U8(recordingDir, encryptionKeyLine, segmentInfoMap, segments)
		if err != nil {
			return "", err
		}

		return tempM3U8Path, nil
	}

	return "", fmt.Errorf("m3u8 file not found")

}

// 암호화 관련 내용 찾기
func (obj *StorageST) extractEncryptionKey(m3u8 string) (string, error) {
	file, err := os.Open(m3u8)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 11 && line[:11] == "#EXT-X-KEY:" {
			return line, nil
		}
	}
	return "", fmt.Errorf("EXT-X-KEY not found")
}

// segment에 해당하는 내용 찾기
func (obj *StorageST) findM3U8ForSegment(m3u8Files []string, segments []SegmentInfo) (map[string]string, error) {
	segmentInfoMap := make(map[string]string) // fileName : duration

	for _, m3u8Path := range m3u8Files {
		file, err := os.Open(m3u8Path)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		// file에서 scanner로 duration 찾기
		obj.findDuration(file, segments, segmentInfoMap)

		if len(segmentInfoMap) == len(segments) {
			return segmentInfoMap, nil
		}
	}

	return nil, nil
}

// duration 검색
func (obj *StorageST) findDuration(file *os.File, segments []SegmentInfo, segmentInfoMap map[string]string) {
	scanner := bufio.NewScanner(file)

	targets := make(map[string]bool)
	for _, segment := range segments {
		targets[segment.FileName] = true
	}

	duration := ""

	// EXTINFO (duration)      -> duration이 먼저 쓰여진다.
	// fileName.ts  (실제 파일)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#EXTINF:") {
			duration = strings.TrimPrefix(line, "#EXTINF:")
			duration = strings.TrimSuffix(duration, ",")
		} else if targets[line] {
			segmentInfoMap[line] = duration
		}

		// 다 찾았으면 탈출
		if len(segments) == len(segmentInfoMap) {
			return
		}
	}
	return
}

// 임시 플레이리스트 생성
func (obj *StorageST) createM3U8(recordingDir, encryptionKey string, segmentInfoMap map[string]string, segments []SegmentInfo) (string, error) {
	// 임시 플레이리스트 파일명 생성
	tempFileName := "temp.m3u8"
	tempPlaylistPath := filepath.Join(recordingDir, tempFileName)

	// m3u8 파일 생성
	file, err := os.Create(tempPlaylistPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// m3u8 헤더 작성
	fmt.Fprintf(file, "#EXTM3U\n")
	fmt.Fprintf(file, "#EXT-X-VERSION:3\n")
	fmt.Fprintf(file, "#EXT-X-TARGETDURATION:12\n")
	fmt.Fprintf(file, "#EXT-X-MEDIA-SEQUENCE:0\n")
	fmt.Fprintf(file, "%s\n", encryptionKey)

	// 세그먼트 정보 작성 (segments 배열 순서대로)
	for _, segment := range segments {
		duration, exists := segmentInfoMap[segment.FileName]
		if !exists {
			duration = "10.0" // 기본값
		}
		fmt.Fprintf(file, "#EXTINF:%s,\n", duration)
		fmt.Fprintf(file, "%s\n", segment.FileName)
	}

	fmt.Fprintf(file, "#EXT-X-ENDLIST\n")
	return tempPlaylistPath, nil
}

// GetRecordingListByDate 특정 날짜의 녹화 파일 목록 조회
func (obj *StorageST) GetRecordingListByDate(streamID, date string) ([]map[string]interface{}, error) {
	// 녹화 디렉토리 경로: recordings/날짜/streamID/
	baseDir := filepath.Join(obj.Server.Maintenance.RetentionRoot, date, streamID)

	// 디렉토리 존재 확인
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return []map[string]interface{}{}, nil // 빈 배열 반환
	}

	var recordings []map[string]interface{}

	// 채널 디렉토리 순회 (0, 1, 2, ...)
	channels, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %v", err)
	}

	for _, channel := range channels {
		if !channel.IsDir() {
			continue
		}

		channelID := channel.Name()
		channelDir := filepath.Join(baseDir, channelID)

		// 각 채널 디렉토리에서 m3u8 파일 찾기
		files, err := filepath.Glob(filepath.Join(channelDir, "*.m3u8"))
		if err != nil {
			// out.Printw(out.LogArg{"module": "recording", "fn": "GetRecordingListByDate", "text": "failed to glob m3u8 files", "channelDir": channelDir, "error": err})
			continue
		}

		// 각 m3u8 파일 정보 추가
		for _, filePath := range files {
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				continue
			}

			m3u8Content, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			// 파일 녹화가 실패한 경우 m3u8 내에 #EXT-X-TARGETDURATION:0 이 포함된다.
			// 정상 파일은 #EXT-X-TARGETDURATION:10 과 같은 양수 값을 가진다.
			if bytes.Contains(m3u8Content, []byte("#EXT-X-TARGETDURATION:0")) {
				continue
			}

			fileName := filepath.Base(filePath)
			// sessionID에서 .m3u8 확장자 제거
			sessionID := strings.TrimSuffix(fileName, ".m3u8")

			// sessionID 형식: YYYYMMDD_HHMMSS
			var startTime string
			if len(sessionID) == 15 { // YYYYMMDD_HHMMSS
				parsedTime, err := time.Parse("20060102_150405", sessionID)
				if err == nil {
					startTime = parsedTime.Format("15:04:05")
				} else {
					startTime = sessionID
				}
			} else {
				startTime = sessionID
			}

			// 상대 경로 생성
			relPath := filepath.Join(date, streamID, channelID, fileName)

			recording := map[string]interface{}{
				"sessionId": sessionID,
				"fileName":  fileName,
				"filePath":  relPath,
				"startTime": startTime,
				"size":      fileInfo.Size(),
				"sizeKB":    float64(fileInfo.Size()) / 1024,
				"channelId": channelID,
			}

			recording["m3u8Content"] = string(m3u8Content)

			recordings = append(recordings, recording)
		}
	}

	// 세션ID(시간)로 정렬
	sort.Slice(recordings, func(i, j int) bool {
		return recordings[i]["sessionId"].(string) < recordings[j]["sessionId"].(string)
	})

	return recordings, nil
}
