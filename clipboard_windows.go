package main

import (
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	openClipboard    = user32.NewProc("OpenClipboard")
	closeClipboard   = user32.NewProc("CloseClipboard")
	emptyClipboard   = user32.NewProc("EmptyClipboard")
	setClipboardData = user32.NewProc("SetClipboardData")
	globalAlloc      = kernel32.NewProc("GlobalAlloc")
	globalFree       = kernel32.NewProc("GlobalFree")
	globalLock       = kernel32.NewProc("GlobalLock")
	globalUnlock     = kernel32.NewProc("GlobalUnlock")
	rtlMoveMemory    = kernel32.NewProc("RtlMoveMemory")
)

func writeClipboard(text string) error {
	data, err := syscall.UTF16FromString(text)
	if err != nil {
		return err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Another process may hold the clipboard briefly.
	opened := false
	for range 20 {
		if r, _, _ := openClipboard.Call(0); r != 0 {
			opened = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !opened {
		return fmt.Errorf("OpenClipboard failed")
	}
	defer closeClipboard.Call()

	if r, _, e := emptyClipboard.Call(); r == 0 {
		return fmt.Errorf("EmptyClipboard: %v", e)
	}

	size := uintptr(len(data)) * unsafe.Sizeof(data[0])
	h, _, e := globalAlloc.Call(gmemMoveable, size)
	if h == 0 {
		return fmt.Errorf("GlobalAlloc: %v", e)
	}
	ptr, _, e := globalLock.Call(h)
	if ptr == 0 {
		globalFree.Call(h)
		return fmt.Errorf("GlobalLock: %v", e)
	}
	rtlMoveMemory.Call(ptr, uintptr(unsafe.Pointer(&data[0])), size)
	globalUnlock.Call(h)

	if r, _, e := setClipboardData.Call(cfUnicodeText, h); r == 0 {
		globalFree.Call(h)
		return fmt.Errorf("SetClipboardData: %v", e)
	}
	return nil
}
