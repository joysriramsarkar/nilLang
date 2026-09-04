//go:build windows

package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

type PROCESSENTRY32 struct {
	Size              uint32
	CntUsage          uint32
	ProcessID         uint32
	DefaultHeapID     uintptr
	ModuleID          uint32
	CntThreads        uint32
	ParentProcessID   uint32
	PriorityClassBase int32
	Flags             uint32
	ExeFile           [260]uint16
}

func getParentProcessName() string {
	ppid := uint32(os.Getppid())
	snapshot, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return ""
	}
	defer syscall.CloseHandle(snapshot)

	var procEntry PROCESSENTRY32
	procEntry.Size = uint32(unsafe.Sizeof(procEntry))

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	process32First := kernel32.NewProc("Process32FirstW")
	process32Next := kernel32.NewProc("Process32NextW")

	ret, _, _ := process32First.Call(uintptr(snapshot), uintptr(unsafe.Pointer(&procEntry)))
	for ret != 0 {
		if procEntry.ProcessID == ppid {
			return strings.ToLower(filepath.Base(syscall.UTF16ToString(procEntry.ExeFile[:])))
		}
		ret, _, _ = process32Next.Call(uintptr(snapshot), uintptr(unsafe.Pointer(&procEntry)))
	}
	return ""
}

func checkExplorerExit() {
	if getParentProcessName() == "explorer.exe" {
		fmt.Print("\n[প্রস্থান করতে Enter চাপুন / Press Enter to exit...]")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
	}
}
