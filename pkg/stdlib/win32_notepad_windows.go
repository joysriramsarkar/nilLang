//go:build windows

package stdlib

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

var (
	u32  = syscall.NewLazyDLL("user32.dll")
	g32  = syscall.NewLazyDLL("gdi32.dll")
	k32  = syscall.NewLazyDLL("kernel32.dll")
	cd32 = syscall.NewLazyDLL("comdlg32.dll")

	procGetModuleHandleW     = k32.NewProc("GetModuleHandleW")
	procRegisterClassExW     = u32.NewProc("RegisterClassExW")
	procCreateWindowExW      = u32.NewProc("CreateWindowExW")
	procDefWindowProcW       = u32.NewProc("DefWindowProcW")
	procShowWindow           = u32.NewProc("ShowWindow")
	procUpdateWindow         = u32.NewProc("UpdateWindow")
	procGetMessageW          = u32.NewProc("GetMessageW")
	procTranslateMessage     = u32.NewProc("TranslateMessage")
	procDispatchMessageW     = u32.NewProc("DispatchMessageW")
	procPostQuitMessage      = u32.NewProc("PostQuitMessage")
	procDestroyWindow        = u32.NewProc("DestroyWindow")
	procMoveWindow           = u32.NewProc("MoveWindow")
	procSetWindowTextW       = u32.NewProc("SetWindowTextW")
	procGetWindowTextW       = u32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW = u32.NewProc("GetWindowTextLengthW")
	procSendMessageW         = u32.NewProc("SendMessageW")
	procCreateMenu           = u32.NewProc("CreateMenu")
	procCreatePopupMenu      = u32.NewProc("CreatePopupMenu")
	procAppendMenuW          = u32.NewProc("AppendMenuW")
	procMessageBoxW          = u32.NewProc("MessageBoxW")
	procLoadCursorW          = u32.NewProc("LoadCursorW")
	procGetClientRect        = u32.NewProc("GetClientRect")
	procSetFocus             = u32.NewProc("SetFocus")
	procGetKeyState          = u32.NewProc("GetKeyState")
	procSetForegroundWindow  = u32.NewProc("SetForegroundWindow")
	procBeginPaint           = u32.NewProc("BeginPaint")
	procEndPaint             = u32.NewProc("EndPaint")
	procFillRect             = u32.NewProc("FillRect")
	procInvalidateRect       = u32.NewProc("InvalidateRect")

	procCreateFontW      = g32.NewProc("CreateFontW")
	procCreateSolidBrush = g32.NewProc("CreateSolidBrush")
	procDeleteObject     = g32.NewProc("DeleteObject")
	procSetBkColor       = g32.NewProc("SetBkColor")
	procSetTextColor     = g32.NewProc("SetTextColor")

	procGetOpenFileNameW = cd32.NewProc("GetOpenFileNameW")
	procGetSaveFileNameW = cd32.NewProc("GetSaveFileNameW")
)

const (
	wsOverlappedWindow = 0x00CF0000
	wsVisible          = 0x10000000
	wsChild            = 0x40000000
	wsVScroll          = 0x00200000
	wsHScroll          = 0x00100000
	wsClipChildren     = 0x02000000

	esMultiline   = 0x0004
	esAutoVScroll = 0x0040
	esAutoHScroll = 0x0080
	esWantReturn  = 0x1000
	esNoHideSel   = 0x0100

	enChange = 0x0300

	wmDestroy        = 0x0002
	wmSize           = 0x0005
	wmSetFocus       = 0x0007
	wmPaint          = 0x000F
	wmClose          = 0x0010
	wmSetFont        = 0x0030
	wmKeyDown        = 0x0100
	wmKeyUp          = 0x0101
	wmCommand        = 0x0111
	wmCtlColorEdit   = 0x0133
	wmCtlColorStatic = 0x0138
	wmLButtonUp      = 0x0202
	wmCut            = 0x0300
	wmCopy           = 0x0301
	wmPaste          = 0x0302
	wmClear          = 0x0303

	emGetSel       = 0x00B0
	emSetSel       = 0x00B1
	emReplaceSel   = 0x00C2
	emUndo         = 0x00C7
	emLineIndex    = 0x00BB
	emLineFromChar = 0x00C9
	emSetMargins   = 0x00D3

	ecLeftMargin  = 0x0001
	ecRightMargin = 0x0002

	mfString    = 0x0000
	mfPopup     = 0x0010
	mfSeparator = 0x0800

	idEditControl = 1001
	idStatusLeft  = 1002
	idStatusRight = 1003

	idMenuNew       = 2001
	idMenuOpen      = 2002
	idMenuSave      = 2003
	idMenuSaveAs    = 2004
	idMenuExit      = 2005
	idMenuUndo      = 2010
	idMenuCut       = 2011
	idMenuCopy      = 2012
	idMenuPaste     = 2013
	idMenuDelete    = 2014
	idMenuSelectAll = 2015
	idMenuTimeDate  = 2016
	idMenuZoomIn    = 2020
	idMenuZoomOut   = 2021
	idMenuZoomReset = 2022
	idMenuAbout     = 2030

	mbYesNoCancel  = 0x00000003
	mbIconQuestion = 0x00000020
	mbIconInfo     = 0x00000040
	idYes          = 6
	idNo           = 7
	idCancel       = 2
)

type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       syscall.Handle
}

type winPoint struct {
	x, y int32
}

type winMsg struct {
	hwnd    syscall.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      winPoint
}

type winRect struct {
	left, top, right, bottom int32
}

type paintStruct struct {
	hdc         syscall.Handle
	fErase      int32
	rcPaint     winRect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type openFileNameW struct {
	lStructSize       uint32
	hwndOwner         syscall.Handle
	hInstance         syscall.Handle
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        uintptr
	dwReserved        uint32
	flagsEx           uint32
}

type minimalNotepad struct {
	hwndMain        syscall.Handle
	hwndEdit        syscall.Handle
	hwndStatusLeft  syscall.Handle
	hwndStatusRight syscall.Handle

	hFontEdit   syscall.Handle
	hFontStatus syscall.Handle
	hBrushWhite syscall.Handle
	hBrushBar   syscall.Handle
	hBrushLine  syscall.Handle

	currentFile string
	appName     string
	modified    bool
	updatingUI  bool
	zoomPct     int
	baseFontPx  int32
}

var activeNP *minimalNotepad

func u16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func getWndText(hwnd syscall.Handle) string {
	length, _, _ := procGetWindowTextLengthW.Call(uintptr(hwnd))
	buf := make([]uint16, length+2)
	procGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), length+1)
	return syscall.UTF16ToString(buf)
}

func setWndText(hwnd syscall.Handle, text string) {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\n", "\r\n")
	procSetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(u16(normalized))))
}

func isInteractiveSession() bool {
	for _, arg := range os.Args[1:] {
		if arg == "--cli" || arg == "--headless" {
			return false
		}
	}
	fi, err := os.Stdin.Stat()
	if err != nil {
		return true
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func makeFont(height int32, weight int32, face string) syscall.Handle {
	h, _, _ := procCreateFontW.Call(
		uintptr(height), 0, 0, 0,
		uintptr(weight),
		0, 0, 0, 1, 0, 0, 5, 0,
		uintptr(unsafe.Pointer(u16(face))),
	)
	return syscall.Handle(h)
}

func runWin32NotesGUI(args []object.Object) (object.Object, error) {
	if !isInteractiveSession() {
		return object.FALSE, nil
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	np := &minimalNotepad{
		appName:    "নীলাং নোটপ্যাড",
		zoomPct:    100,
		baseFontPx: 19,
	}

	var initialText string
	if len(args) > 0 {
		if cfgHash, ok := args[0].(*object.Hash); ok {
			titleKey := (&object.String{Value: "appTitle"}).HashKey()
			if p, ok := cfgHash.Pairs[titleKey]; ok {
				if s, ok := p.Value.(*object.String); ok && s.Value != "" {
					np.appName = s.Value
				}
			}
			textKey := (&object.String{Value: "initialText"}).HashKey()
			if p, ok := cfgHash.Pairs[textKey]; ok {
				if s, ok := p.Value.(*object.String); ok {
					initialText = s.Value
				}
			}
		}
	}

	activeNP = np

	hInst, _, _ := procGetModuleHandleW.Call(0)
	clsName := u16("NilLangMinimalNotepadWnd")
	cursor, _, _ := procLoadCursorW.Call(0, 32512) // IDC_ARROW

	whiteBrush, _, _ := procCreateSolidBrush.Call(0x00FFFFFF) // #FFFFFF pure white paper
	barBrush, _, _ := procCreateSolidBrush.Call(0x00F6F4F3)   // #F3F4F6 subtle status bar
	lineBrush, _, _ := procCreateSolidBrush.Call(0x00E5E0DC)  // #DCE0E5 1px divider line
	np.hBrushWhite = syscall.Handle(whiteBrush)
	np.hBrushBar = syscall.Handle(barBrush)
	np.hBrushLine = syscall.Handle(lineBrush)

	wc := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		style:         0x0003,
		lpfnWndProc:   syscall.NewCallback(minimalNotepadWndProc),
		hInstance:     syscall.Handle(hInst),
		hCursor:       syscall.Handle(cursor),
		hbrBackground: np.hBrushWhite,
		lpszClassName: clsName,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	hMenu := buildMinimalMenuBar()

	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(clsName)),
		uintptr(unsafe.Pointer(u16("শিরোনামহীন - "+np.appName))),
		wsOverlappedWindow|wsVisible|wsClipChildren,
		180, 100, 960, 660,
		0, uintptr(hMenu), hInst, 0,
	)
	np.hwndMain = syscall.Handle(hwnd)

	np.createControls(syscall.Handle(hInst), initialText)

	// Check if a file path was passed as a command-line argument
	for _, arg := range os.Args[1:] {
		if !strings.HasPrefix(arg, "-") {
			if data, err := os.ReadFile(arg); err == nil {
				np.currentFile = arg
				np.updatingUI = true
				setWndText(np.hwndEdit, string(data))
				np.updatingUI = false
				np.modified = false
				break
			}
		}
	}

	np.updateWindowTitle()
	np.updateStatusBar()

	procShowWindow.Call(hwnd, 1)
	procUpdateWindow.Call(hwnd)
	procSetForegroundWindow.Call(hwnd)
	procSetFocus.Call(uintptr(np.hwndEdit))

	var msg winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 || int32(ret) == -1 {
			break
		}

		if msg.message == wmKeyDown {
			ctrlDown, _, _ := procGetKeyState.Call(0x11)  // VK_CONTROL
			shiftDown, _, _ := procGetKeyState.Call(0x10) // VK_SHIFT
			isCtrl := int16(ctrlDown) < 0
			isShift := int16(shiftDown) < 0

			if isCtrl {
				switch msg.wParam {
				case 'N':
					np.actionNew()
					continue
				case 'O':
					np.actionOpen()
					continue
				case 'S':
					if isShift {
						np.actionSaveAs()
					} else {
						np.actionSave()
					}
					continue
				case 'A':
					procSendMessageW.Call(uintptr(np.hwndEdit), emSetSel, 0, ^uintptr(0))
					np.updateStatusBar()
					continue
				case 0xBB, 0x6B: // VK_OEM_PLUS or VK_ADD
					np.adjustZoom(10)
					continue
				case 0xBD, 0x6D: // VK_OEM_MINUS or VK_SUBTRACT
					np.adjustZoom(-10)
					continue
				case '0', 0x60: // 0 or Numpad 0
					np.resetZoom()
					continue
				}
			} else if msg.wParam == 0x74 { // F5
				np.insertTimeDate()
				continue
			}
		}

		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))

		if msg.message == wmKeyUp || msg.message == wmLButtonUp {
			np.updateStatusBar()
		}
	}

	return object.TRUE, nil
}

func buildMinimalMenuBar() syscall.Handle {
	hMenu, _, _ := procCreateMenu.Call()

	hFile, _, _ := procCreatePopupMenu.Call()
	procAppendMenuW.Call(hFile, mfString, idMenuNew, uintptr(unsafe.Pointer(u16("নতুন (New)\tCtrl+N"))))
	procAppendMenuW.Call(hFile, mfString, idMenuOpen, uintptr(unsafe.Pointer(u16("খুলুন... (Open...)\tCtrl+O"))))
	procAppendMenuW.Call(hFile, mfString, idMenuSave, uintptr(unsafe.Pointer(u16("সংরক্ষণ (Save)\tCtrl+S"))))
	procAppendMenuW.Call(hFile, mfString, idMenuSaveAs, uintptr(unsafe.Pointer(u16("নতুন নামে সংরক্ষণ... (Save As...)\tCtrl+Shift+S"))))
	procAppendMenuW.Call(hFile, mfSeparator, 0, 0)
	procAppendMenuW.Call(hFile, mfString, idMenuExit, uintptr(unsafe.Pointer(u16("প্রস্থান (Exit)"))))

	hEdit, _, _ := procCreatePopupMenu.Call()
	procAppendMenuW.Call(hEdit, mfString, idMenuUndo, uintptr(unsafe.Pointer(u16("পূর্বাবস্থা (Undo)\tCtrl+Z"))))
	procAppendMenuW.Call(hEdit, mfSeparator, 0, 0)
	procAppendMenuW.Call(hEdit, mfString, idMenuCut, uintptr(unsafe.Pointer(u16("কাট (Cut)\tCtrl+X"))))
	procAppendMenuW.Call(hEdit, mfString, idMenuCopy, uintptr(unsafe.Pointer(u16("কপি (Copy)\tCtrl+C"))))
	procAppendMenuW.Call(hEdit, mfString, idMenuPaste, uintptr(unsafe.Pointer(u16("পেস্ট (Paste)\tCtrl+V"))))
	procAppendMenuW.Call(hEdit, mfString, idMenuDelete, uintptr(unsafe.Pointer(u16("মুছুন (Delete)\tDel"))))
	procAppendMenuW.Call(hEdit, mfSeparator, 0, 0)
	procAppendMenuW.Call(hEdit, mfString, idMenuSelectAll, uintptr(unsafe.Pointer(u16("সব নির্বাচন (Select All)\tCtrl+A"))))
	procAppendMenuW.Call(hEdit, mfString, idMenuTimeDate, uintptr(unsafe.Pointer(u16("সময় ও তারিখ (Time/Date)\tF5"))))

	hView, _, _ := procCreatePopupMenu.Call()
	procAppendMenuW.Call(hView, mfString, idMenuZoomIn, uintptr(unsafe.Pointer(u16("জুম ইন (Zoom In)\tCtrl++"))))
	procAppendMenuW.Call(hView, mfString, idMenuZoomOut, uintptr(unsafe.Pointer(u16("জুম আউট (Zoom Out)\tCtrl+-"))))
	procAppendMenuW.Call(hView, mfString, idMenuZoomReset, uintptr(unsafe.Pointer(u16("ডিফল্ট জুম (100%)\tCtrl+0"))))

	hHelp, _, _ := procCreatePopupMenu.Call()
	procAppendMenuW.Call(hHelp, mfString, idMenuAbout, uintptr(unsafe.Pointer(u16("নীলাং নোটপ্যাড সম্পর্কে (About)"))))

	procAppendMenuW.Call(hMenu, mfPopup, hFile, uintptr(unsafe.Pointer(u16("ফাইল (File)"))))
	procAppendMenuW.Call(hMenu, mfPopup, hEdit, uintptr(unsafe.Pointer(u16("সম্পাদনা (Edit)"))))
	procAppendMenuW.Call(hMenu, mfPopup, hView, uintptr(unsafe.Pointer(u16("প্রদর্শন (View)"))))
	procAppendMenuW.Call(hMenu, mfPopup, hHelp, uintptr(unsafe.Pointer(u16("সহায়তা (Help)"))))

	return syscall.Handle(hMenu)
}

func (np *minimalNotepad) createControls(hInst syscall.Handle, initialText string) {
	np.hFontEdit = makeFont(-np.baseFontPx, 400, "Nirmala UI")
	np.hFontStatus = makeFont(-14, 400, "Segoe UI")

	// Borderless, padded, full-bleed white EDIT control
	hwEdit, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(u16("EDIT"))),
		uintptr(unsafe.Pointer(u16(""))),
		uintptr(wsChild|wsVisible|wsVScroll|esMultiline|esAutoVScroll|esWantReturn|esNoHideSel),
		16, 12, 910, 560,
		uintptr(np.hwndMain), idEditControl, uintptr(hInst), 0,
	)
	np.hwndEdit = syscall.Handle(hwEdit)
	procSendMessageW.Call(hwEdit, wmSetFont, uintptr(np.hFontEdit), 1)
	// Add generous left & right internal text margins (12px each)
	margins := uintptr((12 << 16) | 12)
	procSendMessageW.Call(hwEdit, emSetMargins, ecLeftMargin|ecRightMargin, margins)

	hwStatusL, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(u16("STATIC"))),
		uintptr(unsafe.Pointer(u16("  লাইন ১, কলাম ১   |   ০ শব্দ, ০ অক্ষর"))),
		uintptr(wsChild|wsVisible),
		0, 580, 540, 26,
		uintptr(np.hwndMain), idStatusLeft, uintptr(hInst), 0,
	)
	np.hwndStatusLeft = syscall.Handle(hwStatusL)
	procSendMessageW.Call(hwStatusL, wmSetFont, uintptr(np.hFontStatus), 1)

	hwStatusR, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(u16("STATIC"))),
		uintptr(unsafe.Pointer(u16("100%   |   Windows (CRLF)   |   UTF-8   "))),
		uintptr(wsChild|wsVisible|0x0002), // SS_RIGHT = 0x0002
		540, 580, 400, 26,
		uintptr(np.hwndMain), idStatusRight, uintptr(hInst), 0,
	)
	np.hwndStatusRight = syscall.Handle(hwStatusR)
	procSendMessageW.Call(hwStatusR, wmSetFont, uintptr(np.hFontStatus), 1)

	if initialText != "" {
		np.updatingUI = true
		setWndText(np.hwndEdit, initialText)
		np.updatingUI = false
		np.modified = false
	}

	var rc winRect
	procGetClientRect.Call(uintptr(np.hwndMain), uintptr(unsafe.Pointer(&rc)))
	np.layout(rc.right-rc.left, rc.bottom-rc.top)
}

func (np *minimalNotepad) layout(w, h int32) {
	if np.hwndEdit == 0 || w < 100 || h < 80 {
		return
	}
	statusH := int32(28)
	topPad := int32(10)
	sidePad := int32(14)

	editW := w - sidePad*2
	editH := h - statusH - topPad - 4
	if editW < 50 {
		editW = 50
	}
	if editH < 40 {
		editH = 40
	}

	procMoveWindow.Call(uintptr(np.hwndEdit), uintptr(sidePad), uintptr(topPad), uintptr(editW), uintptr(editH), 1)

	statusY := h - statusH + 5
	rightW := int32(320)
	leftW := w - rightW
	if leftW < 100 {
		leftW = 100
	}
	procMoveWindow.Call(uintptr(np.hwndStatusLeft), 10, uintptr(statusY), uintptr(leftW-10), uintptr(statusH-5), 1)
	procMoveWindow.Call(uintptr(np.hwndStatusRight), uintptr(leftW), uintptr(statusY), uintptr(rightW-12), uintptr(statusH-5), 1)
	procInvalidateRect.Call(uintptr(np.hwndMain), 0, 1)
}

func (np *minimalNotepad) updateWindowTitle() {
	name := "শিরোনামহীন"
	if np.currentFile != "" {
		name = filepath.Base(np.currentFile)
	}
	prefix := ""
	if np.modified {
		prefix = "*"
	}
	title := fmt.Sprintf("%s%s - %s", prefix, name, np.appName)
	procSetWindowTextW.Call(uintptr(np.hwndMain), uintptr(unsafe.Pointer(u16(title))))
}

func (np *minimalNotepad) updateStatusBar() {
	if np.hwndEdit == 0 {
		return
	}
	var selStart, selEnd uint32
	procSendMessageW.Call(uintptr(np.hwndEdit), emGetSel, uintptr(unsafe.Pointer(&selStart)), uintptr(unsafe.Pointer(&selEnd)))

	lineIdx, _, _ := procSendMessageW.Call(uintptr(np.hwndEdit), emLineFromChar, uintptr(selStart), 0)
	lineStart, _, _ := procSendMessageW.Call(uintptr(np.hwndEdit), emLineIndex, lineIdx, 0)
	colIdx := int(selStart) - int(lineStart) + 1
	if colIdx < 1 {
		colIdx = 1
	}

	text := strings.ReplaceAll(getWndText(np.hwndEdit), "\r\n", "\n")
	words := len(strings.Fields(text))
	chars := utf8.RuneCountInString(text)

	leftStatus := fmt.Sprintf("  লাইন %d, কলাম %d   |   %d শব্দ, %d অক্ষর", int(lineIdx)+1, colIdx, words, chars)
	rightStatus := fmt.Sprintf("%d%%   |   Windows (CRLF)   |   UTF-8  ", np.zoomPct)

	procSetWindowTextW.Call(uintptr(np.hwndStatusLeft), uintptr(unsafe.Pointer(u16(leftStatus))))
	procSetWindowTextW.Call(uintptr(np.hwndStatusRight), uintptr(unsafe.Pointer(u16(rightStatus))))
}

func (np *minimalNotepad) adjustZoom(delta int) {
	newZoom := np.zoomPct + delta
	if newZoom < 50 {
		newZoom = 50
	} else if newZoom > 300 {
		newZoom = 300
	}
	if newZoom == np.zoomPct {
		return
	}
	np.zoomPct = newZoom
	np.applyZoomFont()
}

func (np *minimalNotepad) resetZoom() {
	if np.zoomPct == 100 {
		return
	}
	np.zoomPct = 100
	np.applyZoomFont()
}

func (np *minimalNotepad) applyZoomFont() {
	px := (np.baseFontPx * int32(np.zoomPct)) / 100
	if px < 10 {
		px = 10
	}
	newFont := makeFont(-px, 400, "Nirmala UI")
	procSendMessageW.Call(uintptr(np.hwndEdit), wmSetFont, uintptr(newFont), 1)
	if np.hFontEdit != 0 {
		procDeleteObject.Call(uintptr(np.hFontEdit))
	}
	np.hFontEdit = newFont
	np.updateStatusBar()
}

func (np *minimalNotepad) confirmSaveIfModified() bool {
	if !np.modified {
		return true
	}
	name := "শিরোনামহীন"
	if np.currentFile != "" {
		name = filepath.Base(np.currentFile)
	}
	prompt := fmt.Sprintf("আপনি কি \"%s\"-এর পরিবর্তনগুলো সংরক্ষণ করতে চান?", name)
	ret, _, _ := procMessageBoxW.Call(
		uintptr(np.hwndMain),
		uintptr(unsafe.Pointer(u16(prompt))),
		uintptr(unsafe.Pointer(u16(np.appName))),
		mbYesNoCancel|mbIconQuestion,
	)
	switch int(ret) {
	case idYes:
		return np.actionSave()
	case idNo:
		return true
	default:
		return false
	}
}

func (np *minimalNotepad) actionNew() {
	if !np.confirmSaveIfModified() {
		return
	}
	np.updatingUI = true
	setWndText(np.hwndEdit, "")
	np.updatingUI = false
	np.currentFile = ""
	np.modified = false
	np.updateWindowTitle()
	np.updateStatusBar()
	procSetFocus.Call(uintptr(np.hwndEdit))
}

func (np *minimalNotepad) actionOpen() {
	if !np.confirmSaveIfModified() {
		return
	}
	buf := make([]uint16, 512)
	filter := []uint16{
		'T', 'e', 'x', 't', ' ', 'D', 'o', 'c', 'u', 'm', 'e', 'n', 't', 's', ' ', '(', '*', '.', 't', 'x', 't', ';', '*', '.', 'm', 'd', ';', '*', '.', 'n', 'i', 'l', ')', 0,
		'*', '.', 't', 'x', 't', ';', '*', '.', 'm', 'd', ';', '*', '.', 'n', 'i', 'l', 0,
		'A', 'l', 'l', ' ', 'F', 'i', 'l', 'e', 's', ' ', '(', '*', '.', '*', ')', 0,
		'*', '.', '*', 0, 0,
	}
	ofn := openFileNameW{
		lStructSize: uint32(unsafe.Sizeof(openFileNameW{})),
		hwndOwner:   np.hwndMain,
		lpstrFilter: &filter[0],
		lpstrFile:   &buf[0],
		nMaxFile:    uint32(len(buf)),
		lpstrTitle:  u16("ফাইল খুলুন (Open)"),
		flags:       0x00081000,
	}
	ret, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if ret == 0 {
		return
	}
	path := syscall.UTF16ToString(buf)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	np.updatingUI = true
	setWndText(np.hwndEdit, string(data))
	np.updatingUI = false
	np.currentFile = path
	np.modified = false
	np.updateWindowTitle()
	np.updateStatusBar()
	procSetFocus.Call(uintptr(np.hwndEdit))
}

func (np *minimalNotepad) actionSave() bool {
	if np.currentFile == "" {
		return np.actionSaveAs()
	}
	text := getWndText(np.hwndEdit)
	if err := os.WriteFile(np.currentFile, []byte(text), 0644); err != nil {
		return false
	}
	np.modified = false
	np.updateWindowTitle()
	np.updateStatusBar()
	return true
}

func (np *minimalNotepad) actionSaveAs() bool {
	buf := make([]uint16, 512)
	defaultName := "Untitled.txt"
	if np.currentFile != "" {
		defaultName = filepath.Base(np.currentFile)
	}
	copy(buf, syscall.StringToUTF16(defaultName))
	filter := []uint16{
		'T', 'e', 'x', 't', ' ', 'D', 'o', 'c', 'u', 'm', 'e', 'n', 't', 's', ' ', '(', '*', '.', 't', 'x', 't', ')', 0,
		'*', '.', 't', 'x', 't', 0,
		'A', 'l', 'l', ' ', 'F', 'i', 'l', 'e', 's', ' ', '(', '*', '.', '*', ')', 0,
		'*', '.', '*', 0, 0,
	}
	ofn := openFileNameW{
		lStructSize: uint32(unsafe.Sizeof(openFileNameW{})),
		hwndOwner:   np.hwndMain,
		lpstrFilter: &filter[0],
		lpstrFile:   &buf[0],
		nMaxFile:    uint32(len(buf)),
		lpstrTitle:  u16("সংরক্ষণ করুন (Save As)"),
		lpstrDefExt: u16("txt"),
		flags:       0x00080002,
	}
	ret, _, _ := procGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if ret == 0 {
		return false
	}
	np.currentFile = syscall.UTF16ToString(buf)
	return np.actionSave()
}

func (np *minimalNotepad) insertTimeDate() {
	stamp := time.Now().Format("03:04 PM 02/01/2006")
	procSendMessageW.Call(uintptr(np.hwndEdit), emReplaceSel, 1, uintptr(unsafe.Pointer(u16(stamp))))
	np.modified = true
	np.updateWindowTitle()
	np.updateStatusBar()
}

func (np *minimalNotepad) showAbout() {
	msg := "📝 নীলাং নোটপ্যাড (Nilang Notepad)\n\n" +
		"নীলাং (NilLang) ভাষায় তৈরি পরিচ্ছন্ন ও মিনিমাল উইন্ডোজ নোটপ্যাড।\n\n" +
		"শর্টকাটসমূহ:\n" +
		"• Ctrl+N : নতুন ফাইল\n" +
		"• Ctrl+O : ফাইল খুলুন\n" +
		"• Ctrl+S : সংরক্ষণ করুন\n" +
		"• Ctrl++ / Ctrl+- : ফন্ট জুম ইন / আউট\n" +
		"• F5 : সময় ও তারিখ বসান"
	procMessageBoxW.Call(
		uintptr(np.hwndMain),
		uintptr(unsafe.Pointer(u16(msg))),
		uintptr(unsafe.Pointer(u16("নীলাং নোটপ্যাড সম্পর্কে"))),
		mbIconInfo,
	)
}

func minimalNotepadWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	np := activeNP
	switch msg {
	case wmSize:
		if np != nil {
			var rc winRect
			procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
			np.layout(rc.right-rc.left, rc.bottom-rc.top)
		}
		return 0

	case wmSetFocus:
		if np != nil && np.hwndEdit != 0 {
			procSetFocus.Call(uintptr(np.hwndEdit))
		}
		return 0

	case wmPaint:
		if np != nil {
			var ps paintStruct
			hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
			var rc winRect
			procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
			statusH := int32(28)

			// White paper area
			paperRect := winRect{left: 0, top: 0, right: rc.right, bottom: rc.bottom - statusH}
			procFillRect.Call(hdc, uintptr(unsafe.Pointer(&paperRect)), uintptr(np.hBrushWhite))

			// 1px subtle top border for status bar
			lineRect := winRect{left: 0, top: rc.bottom - statusH, right: rc.right, bottom: rc.bottom - statusH + 1}
			procFillRect.Call(hdc, uintptr(unsafe.Pointer(&lineRect)), uintptr(np.hBrushLine))

			// Status bar background
			barRect := winRect{left: 0, top: rc.bottom - statusH + 1, right: rc.right, bottom: rc.bottom}
			procFillRect.Call(hdc, uintptr(unsafe.Pointer(&barRect)), uintptr(np.hBrushBar))

			procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
			return 0
		}

	case wmCtlColorEdit:
		if np != nil {
			procSetBkColor.Call(wParam, 0x00FFFFFF)
			procSetTextColor.Call(wParam, 0x002A201A) // deep charcoal #1A202A
			return uintptr(np.hBrushWhite)
		}

	case wmCtlColorStatic:
		if np != nil {
			procSetBkColor.Call(wParam, 0x00F6F4F3)
			procSetTextColor.Call(wParam, 0x0064554B) // muted slate #4B5564
			return uintptr(np.hBrushBar)
		}

	case wmCommand:
		if np == nil {
			break
		}
		id := int(wParam & 0xFFFF)
		notifyCode := int((wParam >> 16) & 0xFFFF)

		switch id {
		case idEditControl:
			if notifyCode == enChange && !np.updatingUI {
				if !np.modified {
					np.modified = true
					np.updateWindowTitle()
				}
				np.updateStatusBar()
			}
			return 0
		case idMenuNew:
			np.actionNew()
			return 0
		case idMenuOpen:
			np.actionOpen()
			return 0
		case idMenuSave:
			np.actionSave()
			return 0
		case idMenuSaveAs:
			np.actionSaveAs()
			return 0
		case idMenuExit:
			if np.confirmSaveIfModified() {
				procDestroyWindow.Call(uintptr(hwnd))
			}
			return 0
		case idMenuUndo:
			procSendMessageW.Call(uintptr(np.hwndEdit), emUndo, 0, 0)
			return 0
		case idMenuCut:
			procSendMessageW.Call(uintptr(np.hwndEdit), wmCut, 0, 0)
			return 0
		case idMenuCopy:
			procSendMessageW.Call(uintptr(np.hwndEdit), wmCopy, 0, 0)
			return 0
		case idMenuPaste:
			procSendMessageW.Call(uintptr(np.hwndEdit), wmPaste, 0, 0)
			return 0
		case idMenuDelete:
			procSendMessageW.Call(uintptr(np.hwndEdit), wmClear, 0, 0)
			return 0
		case idMenuSelectAll:
			procSendMessageW.Call(uintptr(np.hwndEdit), emSetSel, 0, ^uintptr(0))
			np.updateStatusBar()
			return 0
		case idMenuTimeDate:
			np.insertTimeDate()
			return 0
		case idMenuZoomIn:
			np.adjustZoom(10)
			return 0
		case idMenuZoomOut:
			np.adjustZoom(-10)
			return 0
		case idMenuZoomReset:
			np.resetZoom()
			return 0
		case idMenuAbout:
			np.showAbout()
			return 0
		}

	case wmClose:
		if np != nil && !np.confirmSaveIfModified() {
			return 0
		}
		procDestroyWindow.Call(uintptr(hwnd))
		return 0

	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}
