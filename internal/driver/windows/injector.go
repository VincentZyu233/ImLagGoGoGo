package windows

import (
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	procKeybdEvent    = user32.NewProc("keybd_event")
	procMapVirtualKey = user32.NewProc("MapVirtualKeyW")
	procSendInput     = user32.NewProc("SendInput")
	openClipboard     = user32.NewProc("OpenClipboard")
	closeClipboard    = user32.NewProc("CloseClipboard")
	emptyClipboard    = user32.NewProc("EmptyClipboard")
	setClipboardData  = user32.NewProc("SetClipboardData")
	globalAlloc       = kernel32.NewProc("GlobalAlloc")
	globalLock        = kernel32.NewProc("GlobalLock")
	globalUnlock      = kernel32.NewProc("GlobalUnlock")
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
	vkMenu    = 0x12 // Alt
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
		time.Sleep(20 * time.Millisecond)
	}
	if !opened {
		return fmt.Errorf("unable to open clipboard (locked by other app)")
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

// SendChatMessage 执行完整的打字时序 (100% 对齐原版 C# 经过实战检验的 keybd_event + 剪贴板粘贴方案):
// 1. 释放所有被物理按住的移动键 (WASD/Shift/Ctrl/Alt/Space)
// 2. 将借口文本写入系统剪贴板
// 3. 模拟按键呼出聊天通道 (Y 或 U)
// 4. 关键延时等待 CS2 Panorama UI 聊天框完全展开并取得键盘焦点 (~250ms)
// 5. 执行 Ctrl+A -> Delete 清理输入框可能存在的残留
// 6. 执行 Ctrl+V 瞬间贴入内容
// 7. 敲击回车 Enter 发送
func (inj *WindowsInjector) SendChatMessage(chatKey string, message string, delayMs int) error {
	if delayMs <= 0 {
		delayMs = 100
	}
	stepDelay := time.Duration(delayMs) * time.Millisecond

	log.Printf("📢 [狡辩触发] 准备发送借口: %q (通道键: %s)", message, chatKey)

	// 1. 释放常用移动/修饰按键
	inj.releaseAllKeys()
	time.Sleep(50 * time.Millisecond)

	// 2. 写入剪贴板
	if err := SetClipboardText(message); err != nil {
		log.Printf("⚠️ 写入剪贴板异常: %v，尝试 Unicode 降级投递", err)
		return inj.fallbackUnicodeSend(chatKey, message, stepDelay)
	}
	log.Printf("📋 借口已安全写入系统剪贴板")
	time.Sleep(30 * time.Millisecond)

	// 3. 呼出聊天框 (默认 'Y' 或 'U')
	vk := byte('Y')
	if strings.ToLower(chatKey) == "u" {
		vk = byte('U')
	} else if len(chatKey) == 1 {
		vk = byte(strings.ToUpper(chatKey)[0])
	}

	log.Printf("⌨️ 正在模拟呼出聊天框 [%c]...", vk)
	inj.keyPress(vk, 50*time.Millisecond)

	// 4. 等待 CS2 Panorama UI 展开并取得焦点 (关键: 至少 250ms)
	waitFocus := stepDelay * 2
	if waitFocus < 250*time.Millisecond {
		waitFocus = 250 * time.Millisecond
	}
	time.Sleep(waitFocus)

	// 5. 清理输入框历史残留 (Ctrl+A -> Delete)
	inj.clearChatInput()
	time.Sleep(stepDelay)

	// 6. 粘贴内容 (Ctrl + V)
	log.Printf("📋 正在模拟 Ctrl+V 粘贴文本...")
	inj.pasteFromClipboard()
	time.Sleep(stepDelay)

	// 7. 模拟回车 Enter 发送
	log.Printf("⏎ 正在敲击 Enter 发送借口...")
	inj.keyPress(vkReturn, 50*time.Millisecond)
	log.Printf("✅ 借口注入流程执行完毕！")

	return nil
}

func (inj *WindowsInjector) keyDown(vk byte) {
	scan, _, _ := procMapVirtualKey.Call(uintptr(vk), 0)
	procKeybdEvent.Call(uintptr(vk), scan, 0, 0)
}

func (inj *WindowsInjector) keyUp(vk byte) {
	scan, _, _ := procMapVirtualKey.Call(uintptr(vk), 0)
	procKeybdEvent.Call(uintptr(vk), scan, uintptr(keyeventfKeyup), 0)
}

func (inj *WindowsInjector) keyPress(vk byte, hold time.Duration) {
	inj.keyDown(vk)
	if hold <= 0 {
		hold = 50 * time.Millisecond
	}
	time.Sleep(hold)
	inj.keyUp(vk)
	time.Sleep(30 * time.Millisecond)
}

func (inj *WindowsInjector) releaseAllKeys() {
	keysToRelease := []byte{
		vkW, vkA, vkS, vkD,
		vkSpace, vkShift, vkControl, vkMenu,
	}
	for _, k := range keysToRelease {
		scan, _, _ := procMapVirtualKey.Call(uintptr(k), 0)
		procKeybdEvent.Call(uintptr(k), scan, uintptr(keyeventfKeyup), 0)
	}
}

func (inj *WindowsInjector) clearChatInput() {
	// 模拟 Ctrl + A
	inj.keyDown(vkControl)
	time.Sleep(40 * time.Millisecond)
	inj.keyPress(vkA, 40*time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	inj.keyUp(vkControl)
	time.Sleep(40 * time.Millisecond)

	// 模拟 Delete
	inj.keyPress(vkDelete, 40*time.Millisecond)
}

func (inj *WindowsInjector) pasteFromClipboard() {
	// 模拟 Ctrl + V
	inj.keyDown(vkControl)
	time.Sleep(40 * time.Millisecond)
	inj.keyPress(vkV, 40*time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	inj.keyUp(vkControl)
}

func (inj *WindowsInjector) fallbackUnicodeSend(chatKey string, message string, interval time.Duration) error {
	vk := uint16('Y')
	if strings.ToLower(chatKey) == "u" {
		vk = uint16('U')
	} else if len(chatKey) == 1 {
		vk = uint16(strings.ToUpper(chatKey)[0])
	}
	inj.keyPress(byte(vk), 50*time.Millisecond)
	time.Sleep(250 * time.Millisecond)

	u16 := utf16.Encode([]rune(message))
	for _, code := range u16 {
		down := input{inputType: inputKeyboard, ki: keyboardInput{wScan: code, dwFlags: keyeventfUnicode}}
		up := input{inputType: inputKeyboard, ki: keyboardInput{wScan: code, dwFlags: keyeventfUnicode | keyeventfKeyup}}
		inputs := []input{down, up}
		procSendInput.Call(uintptr(len(inputs)), uintptr(unsafe.Pointer(&inputs[0])), uintptr(unsafe.Sizeof(inputs[0])))
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(interval)
	inj.keyPress(vkReturn, 50*time.Millisecond)
	return nil
}
