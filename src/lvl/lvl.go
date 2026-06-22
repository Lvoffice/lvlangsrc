// src/lvl/lvl.go - Windows 终端编码适配打印包
package lvl

import (
	"bytes"
	"fmt"
	"io"
	"syscall"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var needGBK bool
var consoleCodePage uint32

// getConsoleCodePage 获取当前终端代码页
func getConsoleCodePage() uint32 {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetConsoleOutputCP")
	ret, _, _ := proc.Call()
	return uint32(ret)
}

// setConsoleOutputCP 设置终端输出代码页
func setConsoleOutputCP(cp uint32) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("SetConsoleOutputCP")
	proc.Call(uintptr(cp))
}

// utf8ToGBK 将 UTF-8 字符串转换为 GBK 字节
func utf8ToGBK(s string) ([]byte, error) {
	reader := transform.NewReader(bytes.NewReader([]byte(s)), simplifiedchinese.GBK.NewEncoder())
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(reader)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Print 自动适配终端编码
func Print(a ...interface{}) {
	fmt.Print(a...)
}

// Println 自动适配终端编码（带换行）
func Println(a ...interface{}) {
	fmt.Println(a...)
}

// Printf 自动适配终端编码的格式化打印
func Printf(format string, a ...interface{}) {
	fmt.Printf(format, a...)
}

// Fprint 写入任意 io.Writer
func Fprint(w io.Writer, a ...interface{}) {
	fmt.Fprint(w, a...)
}

// Fprintln 写入任意 io.Writer（带换行）
func Fprintln(w io.Writer, a ...interface{}) {
	fmt.Fprintln(w, a...)
}

// Fprintf 写入任意 io.Writer（格式化）
func Fprintf(w io.Writer, format string, a ...interface{}) {
	fmt.Fprintf(w, format, a...)
}

// ToGBK 将 UTF-8 字符串转换为 GBK 字节
func ToGBK(s string) []byte {
	gbkBytes, err := utf8ToGBK(s)
	if err != nil {
		return []byte(s)
	}
	return gbkBytes
}

// ToUTF8 将 GBK 字节转换为 UTF-8 字符串
func ToUTF8(b []byte) string {
	reader := transform.NewReader(bytes.NewReader(b), simplifiedchinese.GBK.NewDecoder())
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(reader)
	if err != nil {
		return string(b)
	}
	return buf.String()
}

func init() {
	consoleCodePage = getConsoleCodePage()
	needGBK = (consoleCodePage == 936)

	// 方案：如果终端是 GBK (936)，强制切换到 UTF-8 (65001)
	// 这样 Go 的 UTF-8 输出就能正确显示中文
	if needGBK {
		setConsoleOutputCP(65001)
		needGBK = false
	}
}
