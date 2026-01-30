//go:build linux

package main

import "syscall"

func (obj *StorageST) getFreeDiskSpace() (freeSpaceGB, totalSpaceGB float64, err error) {
	path := obj.Server.Maintenance.RetentionRoot

	var stat syscall.Statfs_t
	err = syscall.Statfs(path, &stat)
	if err != nil {
		return 0, 0, err
	}
	// 사용 가능한 블록 수 * 블록 크기
	freeSpaceBytes := stat.Bavail * uint64(stat.Bsize)

	// 전체 블록 수 * 블록 크기
	totalSpaceBytes := stat.Blocks * uint64(stat.Bsize)

	freeSpaceGB = float64(freeSpaceBytes) / (1024 * 1024 * 1024)
	totalSpaceGB = float64(totalSpaceBytes) / (1024 * 1024 * 1024)

	return freeSpaceGB, totalSpaceGB, nil
}
