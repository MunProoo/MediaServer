//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// Windows API 구조체
type ULARGE_INTEGER struct {
	LowPart  uint32
	HighPart uint32
}

// Windows API 함수 선언
var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")
)

func (obj *StorageST) getFreeDiskSpace() (freeSpaceGB, totalSpaceGB float64, err error) {
	path := obj.Server.Maintenance.RetentionRoot

	// 경로를 UTF16으로 변환
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}

	var freeBytes, totalBytes, totalFreeBytes ULARGE_INTEGER

	// GetDiskFreeSpaceExW 호출
	ret, _, callErr := procGetDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytes)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)

	if ret == 0 {
		return 0, 0, callErr
	}

	// 64비트 값을 int64로 변환
	freeSpaceBytes := int64(freeBytes.HighPart)<<32 | int64(freeBytes.LowPart)
	totalSpaceBytes := int64(totalBytes.HighPart)<<32 | int64(totalBytes.LowPart)

	// 바이트를 GB로 변환
	freeSpaceGB = float64(freeSpaceBytes) / (1024 * 1024 * 1024)
	totalSpaceGB = float64(totalSpaceBytes) / (1024 * 1024 * 1024)

	return freeSpaceGB, totalSpaceGB, nil
}
