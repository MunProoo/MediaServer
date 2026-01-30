package logUtil

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logFile     *os.File
	currentDate string
	logMutex    sync.Mutex
	isService   bool
)

// isServiceMode 서비스 모드인지 확인 (기본값: 자동 감지)
// 표준 출력이 터미널이 아니면 서비스 모드로 간주
func isServiceMode() bool {
	// 이미 설정된 값이 있으면 사용
	if isService {
		return true
	}

	// 표준 출력이 제대로 연결되어 있는지 확인
	fi, err := os.Stdout.Stat()
	if err != nil {
		return true // 표준 출력을 확인할 수 없으면 서비스 모드로 간주
	}

	// 표준 출력이 터미널이 아니면 서비스 모드로 간주
	return (fi.Mode() & os.ModeCharDevice) == 0
}

// getLogFilePath 현재 날짜에 맞는 로그 파일 경로 반환
func getLogFilePath(serverName string) (string, error) {
	// 현재 날짜로 폴더명 생성 (YYYYMMDD)
	dateStr := time.Now().Format("20060102")

	// logs/YYYYMMDD 경로 생성
	logDir := filepath.Join("logs", dateStr)

	// 디렉토리 생성
	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create log directory: %w", err)
	}

	// 기존 로그 파일 확인하여 다음 번호 결정
	logNum := 0
	for {
		logFileName := fmt.Sprintf("%s_%d.log", serverName, logNum)
		logFilePath := filepath.Join(logDir, logFileName)

		// 파일이 존재하지 않으면 이 번호 사용
		if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
			return logFilePath, nil
		}

		// 파일이 존재하면 다음 번호 시도
		logNum++
	}
}

// checkAndRotateLog 날짜가 바뀌었는지 확인하고 필요시 로그 파일 교체
func checkAndRotateLog(serverName string) error {
	logMutex.Lock()
	defer logMutex.Unlock()

	currentDateStr := time.Now().Format("20060102")

	// 날짜가 바뀌지 않았으면 그대로 유지
	if currentDate == currentDateStr && logFile != nil {
		return nil
	}

	// 기존 로그 파일 닫기
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}

	// 새 로그 파일 경로 가져오기
	logFilePath, err := getLogFilePath(serverName)
	if err != nil {
		return err
	}

	// 새 로그 파일 생성
	logFile, err = os.Create(logFilePath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	// 서비스 모드이면 파일에만 기록, 아니면 콘솔과 파일 모두에 기록
	if isService {
		log.SetOutput(logFile)
	} else {
		multiWriter := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(multiWriter)
	}

	currentDate = currentDateStr
	log.Printf("[INFO] [logging] [checkAndRotateLog] Log file created: %s", logFilePath)

	return nil
}

// InitLogging 로그 파일 초기화
// logs/YYYYMMDD/mediaServer_N.log 형식으로 생성
// serviceMode: true면 서비스 모드(파일만), false면 일반 모드(콘솔+파일)
func InitLogging(serverName string, serviceMode ...bool) error {
	// 서비스 모드 설정 (파라미터가 있으면 사용, 없으면 자동 감지)
	if len(serviceMode) > 0 {
		isService = serviceMode[0]
	} else {
		// 자동 감지
		isService = isServiceMode()
	}

	// 초기 로그 파일 생성
	if err := checkAndRotateLog(serverName); err != nil {
		return err
	}

	// 로그 포맷 설정 (날짜/시간 포함)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// 날짜 변경 체크를 위한 고루틴 시작
	go func() {
		ticker := time.NewTicker(1 * time.Minute) // 1분마다 체크
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := checkAndRotateLog(serverName); err != nil {
					// 로그 파일 교체 실패 시 로그 출력
					// 서비스 모드가 아니면 콘솔에도 출력
					if !isService {
						fmt.Fprintf(os.Stderr, "[ERROR] [logging] Failed to rotate log file: %v\n", err)
					}
					// 파일 로그에도 기록 시도 (logFile이 nil이 아닐 수 있음)
					if logFile != nil {
						fmt.Fprintf(logFile, "[ERROR] [logging] Failed to rotate log file: %v\n", err)
					}
				}
			}
		}
	}()

	return nil
}

// CloseLogging 로그 파일 닫기
func CloseLogging() {
	logMutex.Lock()
	defer logMutex.Unlock()

	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}
