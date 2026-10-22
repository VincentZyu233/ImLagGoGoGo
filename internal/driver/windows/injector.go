package windows

import (
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"
)

var (
	procSendInput = user32.NewProc("SendInput")
)

const (
	inputKeyboard        = 1
	keyeventfKeyup       = 0x0002
	keyeventfUnicode     = 0x0004
	vkReturn             = 0x0D
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

// SendChatMessage 执行完整的打字时序: 按下聊天键 -> 延迟 -> Unicode 注入内容 -> 延迟 -> 按下回车
func (inj *WindowsInjector) SendChatMessage(chatKey string, message string, delayMs int) error {
	if delayMs <= 0 {
		delayMs = 60
	}
	interval := time.Duration(delayMs) * time.Millisecond

	// 1. 模拟呼出聊天通道 (默认 'Y' 或 'U')
	vk := uint16('Y')
	if strings.ToLower(chatKey) == "u" {
		vk = uint16('U')
	} else if len(chatKey) == 1 {
		vk = uint16(strings.ToUpper(chatKey)[0])
	}

	inj.pressVirtualKey(vk)
	time.Sleep(interval)

	// 2. 使用 Unicode 码点直接投递字符，彻底免疫输入法候选框
	u16 := utf16.Encode([]rune(message))
	for _, code := range u16 {
		inj.sendUnicodeChar(code)
	}
	time.Sleep(interval)

	// 3. 模拟发送回车 Enter
	inj.pressVirtualKey(vkReturn)

	return nil
}

func (inj *WindowsInjector) pressVirtualKey(vk uint16) {
	down := input{
		inputType: inputKeyboard,
		ki: keyboardInput{
			wVk: vk,
		},
	}
	up := input{
		inputType: inputKeyboard,
		ki: keyboardInput{
			wVk:     vk,
			dwFlags: keyeventfKeyup,
		},
	}

	inputs := []input{down, up}
	procSendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs[0])),
		uintptr(unsafe.Sizeof(inputs[0])),
	)
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
