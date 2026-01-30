package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// 파일 정보 구조체
type FileInfo struct {
	Path    string
	Size    int64
	ModTime time.Time
}

// 디렉토리 내 모든 파일 스캔
func scanDirectory(dirPath string) ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// 디렉토리는 건너뛰기
		if d.IsDir() {
			return nil
		}

		// 파일 정보 가져오기
		info, err := d.Info()
		if err != nil {
			return err
		}

		// 숨김 파일이나 시스템 파일 제외
		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}

		files = append(files, FileInfo{
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})

		return nil
	})

	return files, err
}

// 파일들을 수정 시간순으로 정렬 (오래된 것부터)
func sortFilesByModTime(files []FileInfo) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.Before(files[j].ModTime)
	})
}

// 지정된 용량만큼 가장 오래된 파일들 삭제 (시나리오 2: 사용 가능한 모든 파일 삭제)
func deleteOldFiles(files []FileInfo, targetSizeGB int64) (deletedSize int64, deletedCount int) {
	targetSizeBytes := targetSizeGB * 1024 * 1024 * 1024 // GB를 바이트로 변환

	for _, file := range files {
		if deletedSize >= targetSizeBytes {
			break
		}

		// 파일 삭제
		err := os.Remove(file.Path)
		if err != nil {
			log.Printf("파일 삭제 실패: %s, 오류: %v", file.Path, err)
			continue
		}

		deletedSize += file.Size
		deletedCount++
	}

	if deletedSize < targetSizeBytes {
		text := fmt.Sprintf("Warning: Failed to reach target size of %.2f GB. Deleted all available files.", float64(targetSizeBytes)/(1024*1024*1024))
		log.Println(text)
	}

	return deletedSize, deletedCount
}

type DayFolder struct {
	Path      string
	Date      time.Time
	TotalSize int64
}

// 날짜 디렉토리 목록 반환
func listRecordingDayFolders(root string) ([]DayFolder, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var folders []DayFolder
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dayStr := entry.Name() // 2025-11-11
		dayTime, err := time.Parse("2006-01-02", dayStr)
		if err != nil {
			// 날짜형식 아니면 무시
			continue
		}

		path := filepath.Join(root, entry.Name())
		totalSize, err := calculateFolderSize(path)
		if err != nil {
			return nil, err
		}
		folders = append(folders, DayFolder{
			Path:      path,
			Date:      dayTime,
			TotalSize: totalSize,
		})
	}

	// 오래된 순 정렬
	sort.Slice(folders, func(i int, j int) bool {
		return folders[i].Date.Before(folders[j].Date)
	})
	return folders, nil
}

func calculateFolderSize(path string) (int64, error) {
	var total int64
	err := filepath.Walk(path, func(_ string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})

	return total, err
}
