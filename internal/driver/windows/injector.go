package windows

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	procSendInput    = user32.NewProc("SendInput")
	openClipboard    = user32.NewProc("OpenClipboard")
	closeClipboard   = user32.NewProc("CloseClipboard")
	emptyClipboard   = user32.NewProc("EmptyClipboard")
	setClipboardData = user32.NewProc("SetClipboardData")
	globalAlloc      = kernel32.NewProc("GlobalAlloc")
	globalLock       = kernel32.NewProc("GlobalLock")
	globalUnlock     = kernel32.NewProc("GlobalUnlock")
)

const (
	inputKeyboard    = 1
	keyeventfKeyup   = 0x0002
	keyeventfUnicode = 0x0004

	cfUnicodeText = 13
	gmemMoveable  = 0x0002

	vkBack    = 0x08
	vkReturn  = 0x0D
	vkShift   = 0x10
	vkControl = 0x11
	vkSpace   = 0x20
	vkDelete  = 0x2E
	vkA       = 0x41
	vkD       = 0x44
	vkS       = 0x53
	vkV       = 0x56
	vkW       = 0x57
)

type keyboardInput struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

type input struct {
	inputType uint32
	ki        keyboardInput
	padding   [8]byte
}

type WindowsInjector struct{}

func NewWindowsInjector() *WindowsInjector {
	return &WindowsInjector{}
}

// SetClipboardText 将指定文本安全写入 Windows 剪贴板 (带重试锁)
func SetClipboardText(text string) error {
	var opened bool
	for i := 0; i < 5; i++ {
		r, _, _ := openClipboard.Call(0)
		if r != 0 {
			opened = true
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	if !opened {
		return fmt.Errorf("unable to open clipboard")
	}
	defer closeClipboard.Call()

	emptyClipboard.Call()

	u16 := utf16.Encode([]rune(text + "\x00"))
	bytesNeeded := uintptr(len(u16) * 2)

	hMem, _, _ := globalAlloc.Call(gmemMoveable, bytesNeeded)
	if hMem == 0 {
		return fmt.Errorf("globalAlloc failed")
	}

	ptr, _, _ := globalLock.Call(hMem)
	if ptr == 0 {
		return fmt.Errorf("globalLock failed")
	}

	dest := unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(u16))
	copy(dest, u16)

	globalUnlock.Call(hMem)

	r, _, err := setClipboardData.Call(cfUnicodeText, hMem)
	if r == 0 {
		return fmt.Errorf("setClipboardData failed: %v", err)
	}
	return nil
}

// SendChatMessage 执行完整的打字时序:
// 1. 写入剪贴板
// 2. 释放可能被物理长按的移动/修饰键 (WASD/Shift/Ctrl)
// 3. 模拟呼出聊天通道 (Y 或 U)
// 4. 等待 CS2 Panorama 聊天框弹出并获得焦点 (~180-250ms)
// 5. 清理输入框历史残留 (Ctrl+A -> Delete)
// 6. 模拟 Ctrl+V 粘贴内容
// 7. 等待并模拟 Enter 回车发送
func (inj *WindowsInjector) SendChatMessage(chatKey string, message string, delayMs int) error {
	if delayMs <= 0 {
		delayMs = 60
	}
	interval := time.Duration(delayMs) * time.Millisecond

	// 1. 尝试将消息写入剪贴板 (CS2 Panorama UI 完美原生支持剪贴板粘贴)
	useClipboard := true
	if err := SetClipboardText(message); err != nil {
		useClipboard = false
	}

	// 2. 释放常态按键，避免玩家被打死时手指按住 W/Shift 干扰后续输入
	inj.releaseKeys(vkW, vkA, vkS, vkD, vkShift, vkControl, vkSpace)
	time.Sleep(50 * time.Millisecond)

	// 3. 模拟呼出聊天通道 (默认 'Y' 或 'U')
	vk := uint16('Y')
	if strings.ToLower(chatKey) == "u" {
		vk = uint16('U')
	} else if len(chatKey) == 1 {
		vk = uint16(strings.ToUpper(chatKey)[0])
	}

	inj.pressKey(vk, 30*time.Millisecond)

	// 4. 等待 CS2 Panorama UI 聊天框打开并完全获得键盘焦点 (关键等待)
	waitFocus := interval * 3
	if waitFocus < 180*time.Millisecond {
		waitFocus = 180 * time.Millisecond
	}
	time.Sleep(waitFocus)

	// 5. 注入内容: 优先使用 Ctrl+V 粘贴 (稳如泰山)，失败则回退至 SendInput Unicode
	if useClipboard {
		inj.clearChatInput()
		inj.pasteClipboard()
	} else {
		u16 := utf16.Encode([]rune(message))
		for _, code := range u16 {
			inj.sendUnicodeChar(code)
		}
	}

	// 等待 CS2 渲染聊天输入框中的文字
	waitSend := interval
	if waitSend < 80*time.Millisecond {
		waitSend = 80 * time.Millisecond
	}
	time.Sleep(waitSend)

	// 6. 模拟回车 Enter 发送
	inj.pressKey(vkReturn, 30*time.Millisecond)

	return nil
}

// clearChatInput 清除当前聊天框可能残留的字符 (Ctrl+A -> Delete)
func (inj *WindowsInjector) clearChatInput() {
	downCtrl := input{
		inputType: inputKeyboard,
		ki: keyboardInput{
			wVk: vkControl,
		},
	}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&downCtrl)), uintptr(unsafe.Sizeof(downCtrl)))
	time.Sleep(15 * time.Millisecond)

	inj.pressKey(vkA, 20*time.Millisecond)
	time.Sleep(15 * time.Millisecond)

	upCtrl := input{
		inputType: inputKeyboard,
		ki: keyboardInput{
			wVk:     vkControl,
			dwFlags: keyeventfKeyup,
		},
	}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&upCtrl)), uintptr(unsafe.Sizeof(upCtrl)))
	time.Sleep(20 * time.Millisecond)

	inj.pressKey(vkDelete, 20*time.Millisecond)
	time.Sleep(25 * time.Millisecond)
}

// pressKey 模拟按键按下并保持一段时间后释放
func (inj *WindowsInjector) pressKey(vk uint16, hold time.Duration) {
	down := input{
		inputType: inputKeyboard,
		ki: keyboardInput{
			wVk: vk,
		},
	}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&down)), uintptr(unsafe.Sizeof(down)))

	if hold > 0 {
		time.Sleep(hold)
	}

	up := input{
		inputType: inputKeyboard,
		ki: keyboardInput{
			wVk:     vk,
			dwFlags: keyeventfKeyup,
		},
	}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&up)), uintptr(unsafe.Sizeof(up)))
}

// releaseKeys 强制释放一组按键
func (inj *WindowsInjector) releaseKeys(vks ...uint16) {
	for _, vk := range vks {
		up := input{
			inputType: inputKeyboard,
			ki: keyboardInput{
				wVk:     vk,
				dwFlags: keyeventfKeyup,
			},
		}
		procSendInput.Call(1, uintptr(unsafe.Pointer(&up)), uintptr(unsafe.Sizeof(up)))
	}
}

// pasteClipboard 模拟按下 Ctrl+V 进行粘贴
func (inj *WindowsInjector) pasteClipboard() {
	downCtrl := input{
		inputType: inputKeyboard,
		ki: keyboardInput{
			wVk: vkControl,
		},
	}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&downCtrl)), uintptr(unsafe.Sizeof(downCtrl)))
	time.Sleep(20 * time.Millisecond)

	inj.pressKey(vkV, 25*time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	upCtrl := input{
		inputType: inputKeyboard,
		ki: keyboardInput{
			wVk:     vkControl,
			dwFlags: keyeventfKeyup,
		},
	}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&upCtrl)), uintptr(unsafe.Sizeof(upCtrl)))
}

func (inj *WindowsInjector) sendUnicodeChar(code uint16) {
	down := input{
		inputType: inputKeyboard,
		ki: keyboardInput{
			wScan:   code,
			dwFlags: keyeventfUnicode,
		},
	}
	up := input{
		inputType: inputKeyboard,
		ki: keyboardInput{
			wScan:   code,
			dwFlags: keyeventfUnicode | keyeventfKeyup,
		},
	}

	inputs := []input{down, up}
	procSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		uintptr(unsafe.Sizeof(inputs[0])),
	)
}
