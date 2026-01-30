package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

// 메인 유지보수 매니저
func (obj *StorageST) MaintenanceManager() {
	curTime := time.Now()

	// 디스크 체크 타이머
	diskTicker := time.NewTicker(obj.Server.Maintenance.DiskCheckInterval * time.Hour)
	defer diskTicker.Stop()

	// 자정 체크 타이머 (1분마다)
	midnightTicker := time.NewTicker(time.Minute * 1)
	expireDay := curTime.AddDate(0, 0, 0).Day()
	defer midnightTicker.Stop()

	// // 일일 정리 타이머 (매일 새벽 2시)
	// dailyTicker := time.NewTicker(1 * time.Hour)
	// defer dailyTicker.Stop()
	// lastDailyCleanup := time.Now()

	// 녹화 파일 경로 설정 -> NewStreamCore로 위치 변경
	// obj.setRetentionRoot()

	for {
		select {
		case <-diskTicker.C:
			// 디스크 용량 체크 및 정리
			// obj.checkDiskSpaceAndCleanup()
			obj.checkRetentionPolicies()

		case <-midnightTicker.C:
			// 자정 체크 (00:00:00 ~ 00:00:59)
			checkTime := time.Now()
			if expireDay != checkTime.Day() {
				expireDay = checkTime.Day()

				log.Printf("[INFO] [maintenance] [maintenanceManager] midnight - all recording restart")

				obj.AllStreamRestartRecording()
			}

		}
	}
}

func (obj *StorageST) checkRetentionPolicies() {
	cfg := obj.Server.Maintenance
	root := obj.Server.Maintenance.RetentionRoot

	// 1) 보관 용량 초과 정리
	if cfg.RetentionCapacity > 0 {
		obj.purgeRetentionCapacity(root, cfg.RetentionCapacity)
	}

	// 2) 보관 기간 초과 정리
	if cfg.RetentionDays > 0 {
		obj.purgeRetentionDays(root, cfg.RetentionDays)
	}

	obj.ensureMinimumFreeSpace(root, cfg.DefaultSafetyFreeSpace)
}

// 보관 기간에 따른 정리
func (obj *StorageST) purgeRetentionDays(root string, retentionDays int) {
	folders, err := listRecordingDayFolders(root)
	if err != nil {
		log.Printf("[ERROR] [maintenance] [purgeRetentionDays] failed to listRecordingDayFolders: err=%v", err)
		return
	}

	if len(folders) == 0 {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	for _, folder := range folders {
		if folder.Date.Before(cutoff) {
			err := os.RemoveAll(folder.Path)
			if err != nil {
				log.Printf("[ERROR] [maintenance] [purgeRetentionDays] failed to delete expired folder: path=%s err=%v", folder.Path, err)
				continue
			}
			log.Printf("[INFO] [maintenance] [purgeRetentionDays] deleted expired folder: path=%s", folder.Path)
		}
	}
}

// 보관 용량에 따른 정리
func (obj *StorageST) purgeRetentionCapacity(root string, retentionCapacity float64) {
	folders, err := listRecordingDayFolders(root)
	if err != nil {
		log.Printf("[ERROR] [maintenance] [purgeRetentionCapacity] failed to listRecordingDayFolders: err=%v", err)
		return
	}

	if len(folders) == 0 {
		return
	}

	var totalSize int64
	for _, folder := range folders {
		totalSize += folder.TotalSize
	}

	limitBytes := int64(retentionCapacity * 1024 * 1024 * 1024)
	if totalSize <= limitBytes {
		return
	}

	for _, folder := range folders {
		if totalSize <= limitBytes {
			break
		}

		if err := os.RemoveAll(folder.Path); err != nil {
			log.Printf("[ERROR] [maintenance] [purgeRetentionCapacity] failed to delete folder: path=%s err=%v", folder.Path, err)
			continue
		}

		totalSize -= folder.TotalSize
		deletedSize := fmt.Sprintf("%.2fGB", float64(folder.TotalSize)/(1024*1024*1024))
		log.Printf("[INFO] [maintenance] [purgeRetentionCapacity] deleted oldest folder to reduce capacity: path=%s size=%s", folder.Path, deletedSize)
	}
}

func (obj *StorageST) ensureMinimumFreeSpace(root string, minFreeSpaceGB float64) {
	if minFreeSpaceGB <= 0 {
		return
	}

	for {
		freeSpaceGB, _, err := obj.getFreeDiskSpace()
		if err != nil {
			log.Printf("[ERROR] [maintenance] [ensureMinimumFreeSpace] failed to read disk free space: err=%v", err)
			return
		}

		if freeSpaceGB >= minFreeSpaceGB {
			return
		}

		folders, err := listRecordingDayFolders(root)
		if err != nil {
			log.Printf("[ERROR] [maintenance] [ensureMinimumFreeSpace] failed to list recording folders: err=%v", err)
			return
		}

		if len(folders) == 0 {
			log.Printf("[WARN] [maintenance] [ensureMinimumFreeSpace] no folders available to delete despite low free space: freeSpaceGB=%.2fGB", freeSpaceGB)
			return
		}

		oldest := folders[0]
		if err := os.RemoveAll(oldest.Path); err != nil {
			log.Printf("[ERROR] [maintenance] [ensureMinimumFreeSpace] failed to remove folder while freeing space: path=%s err=%v", oldest.Path, err)
			return
		}
		deletedSize := fmt.Sprintf("%.2fGB", float64(oldest.TotalSize)/(1024*1024*1024))
		log.Printf("[WARN] [maintenance] [ensureMinimumFreeSpace] deleted oldest folder due to low disk space: path=%s size=%s freeSpaceGB=%.2fGB", oldest.Path, deletedSize, freeSpaceGB)
	}
}
