package screenshot

import (
	"errors"
	"github.com/liconglei/screenshot/internal/util"
	win "github.com/lxn/win"
	"image"
	"syscall"
	"unsafe"
)

var (
	libUser32, _                 = syscall.LoadLibrary("user32.dll")
	libDwmApi, _                 = syscall.LoadLibrary("dwmapi.dll")
	funcGetDesktopWindow, _      = syscall.GetProcAddress(syscall.Handle(libUser32), "GetDesktopWindow")
	funcEnumDisplayMonitors, _   = syscall.GetProcAddress(syscall.Handle(libUser32), "EnumDisplayMonitors")
	funcGetMonitorInfo, _        = syscall.GetProcAddress(syscall.Handle(libUser32), "GetMonitorInfoW")
	funcEnumDisplaySettings, _   = syscall.GetProcAddress(syscall.Handle(libUser32), "EnumDisplaySettingsW")
	funcSetProcessDPIAware, _    = syscall.GetProcAddress(syscall.Handle(libUser32), "SetProcessDPIAware")
	funcDwmGetWindowAttribute, _ = syscall.GetProcAddress(syscall.Handle(libDwmApi), "DwmGetWindowAttribute")
)

// CaptureScreen Capture whole screen include taskbar.
func CaptureScreen() (*image.RGBA, error) {
	width := int(win.GetSystemMetrics(win.SM_CXSCREEN))
	height := int(win.GetSystemMetrics(win.SM_CYSCREEN))
	return Capture(0, 0, width, height)
}

func CaptureWindow(windowName string) (*image.RGBA, error) {
	lpWindowName, err := syscall.UTF16PtrFromString(windowName)
	if err != nil {
		panic(err)
	}

	hwndWindow := win.FindWindow(nil, lpWindowName)
	if hwndWindow == 0 {
		return nil, errors.New("invalid hwnd")
	}

	SetProcessDPIAware()

	bounds := new(win.RECT)
	if DwmGetWindowAttribute(hwndWindow, DWMWA_EXTENDED_FRAME_BOUNDS, uintptr(unsafe.Pointer(bounds)),
		unsafe.Sizeof(*bounds)) != nil {
		return nil, errors.New("DwmGetWindowAttribute failed")
	}

	width := int(bounds.Right - bounds.Left)
	height := int(bounds.Bottom - bounds.Top)
	return Capture(int(bounds.Left), int(bounds.Top), width, height)
}

func Capture(x, y, width, height int) (*image.RGBA, error) {
	rect := image.Rect(0, 0, width, height)
	img, err := util.CreateImage(rect)
	if err != nil {
		return nil, err
	}

	hwnd := getDesktopWindow()
	hdc := win.GetDC(hwnd)
	if hdc == 0 {
		return nil, errors.New("GetDC failed")
	}
	defer win.ReleaseDC(hwnd, hdc)

	memory_device := win.CreateCompatibleDC(hdc)
	if memory_device == 0 {
		return nil, errors.New("CreateCompatibleDC failed")
	}
	defer win.DeleteDC(memory_device)

	bitmap := win.CreateCompatibleBitmap(hdc, int32(width), int32(height))
	if bitmap == 0 {
		return nil, errors.New("CreateCompatibleBitmap failed")
	}
	defer win.DeleteObject(win.HGDIOBJ(bitmap))

	var header win.BITMAPINFOHEADER
	header.BiSize = uint32(unsafe.Sizeof(header))
	header.BiPlanes = 1
	header.BiBitCount = 32
	header.BiWidth = int32(width)
	header.BiHeight = int32(-height)
	header.BiCompression = win.BI_RGB
	header.BiSizeImage = 0

	// GetDIBits balks at using Go memory on some systems. The MSDN example uses
	// GlobalAlloc, so we'll do that too. See:
	// https://docs.microsoft.com/en-gb/windows/desktop/gdi/capturing-an-image
	bitmapDataSize := uintptr(((int64(width)*int64(header.BiBitCount) + 31) / 32) * 4 * int64(height))
	hmem := win.GlobalAlloc(win.GMEM_MOVEABLE, bitmapDataSize)
	defer win.GlobalFree(hmem)
	memptr := win.GlobalLock(hmem)
	defer win.GlobalUnlock(hmem)

	old := win.SelectObject(memory_device, win.HGDIOBJ(bitmap))
	if old == 0 {
		return nil, errors.New("SelectObject failed")
	}
	defer win.SelectObject(memory_device, old)

	if !win.BitBlt(memory_device, 0, 0, int32(width), int32(height), hdc, int32(x), int32(y), win.SRCCOPY) {
		return nil, errors.New("BitBlt failed")
	}

	if win.GetDIBits(hdc, bitmap, 0, uint32(height), (*uint8)(memptr), (*win.BITMAPINFO)(unsafe.Pointer(&header)), win.DIB_RGB_COLORS) == 0 {
		return nil, errors.New("GetDIBits failed")
	}

	i := 0
	src := uintptr(memptr)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			v0 := *(*uint8)(unsafe.Pointer(src))
			v1 := *(*uint8)(unsafe.Pointer(src + 1))
			v2 := *(*uint8)(unsafe.Pointer(src + 2))

			// BGRA => RGBA, and set A to 255
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = v2, v1, v0, 255

			i += 4
			src += 4
		}
	}

	return img, nil
}

func NumActiveDisplays() int {
	var count int = 0
	enumDisplayMonitors(win.HDC(0), nil, syscall.NewCallback(countupMonitorCallback), uintptr(unsafe.Pointer(&count)))
	return count
}

func GetDisplayBounds(displayIndex int) image.Rectangle {
	var ctx getMonitorBoundsContext
	ctx.Index = displayIndex
	ctx.Count = 0
	enumDisplayMonitors(win.HDC(0), nil, syscall.NewCallback(getMonitorBoundsCallback), uintptr(unsafe.Pointer(&ctx)))
	return image.Rect(
		int(ctx.Rect.Left), int(ctx.Rect.Top),
		int(ctx.Rect.Right), int(ctx.Rect.Bottom))
}

const (
	DWMWA_NCRENDERING_ENABLED         = 1
	DWMWA_NCRENDERING_POLICY          = 2
	DWMWA_TRANSITIONS_FORCEDISABLED   = 3
	DWMWA_ALLOW_NCPAINT               = 4
	DWMWA_CAPTION_BUTTON_BOUNDS       = 5
	DWMWA_NONCLIENT_RTL_LAYOUT        = 6
	DWMWA_FORCE_ICONIC_REPRESENTATION = 7
	DWMWA_FLIP3D_POLICY               = 8
	DWMWA_EXTENDED_FRAME_BOUNDS       = 9
	DWMWA_HAS_ICONIC_BITMAP           = 10
	DWMWA_DISALLOW_PEEK               = 11
	DWMWA_EXCLUDED_FROM_PEEK          = 12
	DWMWA_CLOAK                       = 13
	DWMWA_CLOAKED                     = 14
	DWMWA_FREEZE_REPRESENTATION       = 15
	DWMWA_LAST                        = 16
)

func SetProcessDPIAware() bool {
	ret, _, _ := syscall.Syscall(funcSetProcessDPIAware, 0, 0, 0, 0)
	return int(ret) != 0
}

func DwmGetWindowAttribute(hwnd win.HWND, attribute uint32, value uintptr, size uintptr) (ret error) {
	r0, _, _ := syscall.Syscall6(funcDwmGetWindowAttribute, 4, uintptr(hwnd), uintptr(attribute), value,
		size, 0, 0)
	if r0 != 0 {
		ret = syscall.Errno(r0)
	}
	return
}

func getDesktopWindow() win.HWND {
	ret, _, _ := syscall.Syscall(funcGetDesktopWindow, 0, 0, 0, 0)
	return win.HWND(ret)
}

func enumDisplayMonitors(hdc win.HDC, lprcClip *win.RECT, lpfnEnum uintptr, dwData uintptr) bool {
	ret, _, _ := syscall.Syscall6(funcEnumDisplayMonitors, 4,
		uintptr(hdc),
		uintptr(unsafe.Pointer(lprcClip)),
		lpfnEnum,
		dwData,
		0,
		0)
	return int(ret) != 0
}

func countupMonitorCallback(hMonitor win.HMONITOR, hdcMonitor win.HDC, lprcMonitor *win.RECT, dwData uintptr) uintptr {
	var count *int
	count = (*int)(unsafe.Pointer(dwData))
	*count = *count + 1
	return uintptr(1)
}

type getMonitorBoundsContext struct {
	Index int
	Rect  win.RECT
	Count int
}

func getMonitorBoundsCallback(hMonitor win.HMONITOR, hdcMonitor win.HDC, lprcMonitor *win.RECT, dwData uintptr) uintptr {
	var ctx *getMonitorBoundsContext
	ctx = (*getMonitorBoundsContext)(unsafe.Pointer(dwData))
	if ctx.Count != ctx.Index {
		ctx.Count = ctx.Count + 1
		return uintptr(1)
	}

	if realSize := getMonitorRealSize(hMonitor); realSize != nil {
		ctx.Rect = *realSize
	} else {
		ctx.Rect = *lprcMonitor
	}

	return uintptr(0)
}

type _MONITORINFOEX struct {
	win.MONITORINFO
	DeviceName [win.CCHDEVICENAME]uint16
}

const _ENUM_CURRENT_SETTINGS = 0xFFFFFFFF

type _DEVMODE struct {
	_            [68]byte
	DmSize       uint16
	_            [6]byte
	DmPosition   win.POINT
	_            [86]byte
	DmPelsWidth  uint32
	DmPelsHeight uint32
	_            [40]byte
}

// getMonitorRealSize makes a call to GetMonitorInfo
// to obtain the device name for the monitor handle
// provided to the method.
//
// With the device name, EnumDisplaySettings is called to
// obtain the current configuration for the monitor, this
// information includes the real resolution of the monitor
// rather than the scaled version based on DPI.
//
// If either handle calls fail, it will return a nil
// allowing the calling method to use the bounds information
// returned by EnumDisplayMonitors which may be affected
// by DPI.
func getMonitorRealSize(hMonitor win.HMONITOR) *win.RECT {
	info := _MONITORINFOEX{}
	info.CbSize = uint32(unsafe.Sizeof(info))

	ret, _, _ := syscall.Syscall(funcGetMonitorInfo, 2, uintptr(hMonitor), uintptr(unsafe.Pointer(&info)), 0)
	if ret == 0 {
		return nil
	}

	devMode := _DEVMODE{}
	devMode.DmSize = uint16(unsafe.Sizeof(devMode))

	if ret, _, _ := syscall.Syscall(funcEnumDisplaySettings, 3, uintptr(unsafe.Pointer(&info.DeviceName[0])), _ENUM_CURRENT_SETTINGS, uintptr(unsafe.Pointer(&devMode))); ret == 0 {
		return nil
	}

	return &win.RECT{
		Left:   devMode.DmPosition.X,
		Right:  devMode.DmPosition.X + int32(devMode.DmPelsWidth),
		Top:    devMode.DmPosition.Y,
		Bottom: devMode.DmPosition.Y + int32(devMode.DmPelsHeight),
	}
}
