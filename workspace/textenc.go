package workspace

import (
	"bytes"
	"unicode/utf8"
	"unicode/utf16"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// DecodeTextBytes 将磁盘字节解成 UTF-8 文本。
// 顺序：去 BOM → 合法 UTF-8 → UTF-16 → GB18030 → GBK → 原样。
// 古龙等网文 txt 常见 GBK/GB18030；按 UTF-8 硬读会满屏 �。
func DecodeTextBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if s, ok := decodeWithBOM(b); ok {
		return s
	}
	if utf8.Valid(b) {
		return string(b)
	}
	if s, ok := tryGB(b); ok {
		return s
	}
	return string(b)
}

// NeedsUTF8Rewrite 磁盘字节是否应转写为 UTF-8（导入/归一化用）。
func NeedsUTF8Rewrite(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	if hasTextBOM(b) {
		return true
	}
	return !utf8.Valid(b)
}

func hasTextBOM(b []byte) bool {
	return bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) ||
		bytes.HasPrefix(b, []byte{0xFF, 0xFE}) ||
		bytes.HasPrefix(b, []byte{0xFE, 0xFF})
}

func decodeWithBOM(b []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}):
		rest := b[3:]
		if utf8.Valid(rest) {
			return string(rest), true
		}
		return "", false
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE}):
		return decodeUTF16(b[2:], false), true
	case bytes.HasPrefix(b, []byte{0xFE, 0xFF}):
		return decodeUTF16(b[2:], true), true
	default:
		return "", false
	}
}

func decodeUTF16(b []byte, bigEndian bool) string {
	if len(b)%2 == 1 {
		b = b[:len(b)-1]
	}
	u16 := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		var v uint16
		if bigEndian {
			v = uint16(b[i])<<8 | uint16(b[i+1])
		} else {
			v = uint16(b[i+1])<<8 | uint16(b[i])
		}
		u16 = append(u16, v)
	}
	return string(utf16.Decode(u16))
}

func tryGB(b []byte) (string, bool) {
	for _, enc := range []transform.Transformer{
		simplifiedchinese.GB18030.NewDecoder(),
		simplifiedchinese.GBK.NewDecoder(),
	} {
		out, _, err := transform.Bytes(enc, b)
		if err != nil || len(out) == 0 || !utf8.Valid(out) {
			continue
		}
		if cjkCount(out) == 0 && len(b) > 64 {
			continue
		}
		return string(out), true
	}
	return "", false
}

func cjkCount(b []byte) int {
	n := 0
	for _, r := range string(b) {
		if r >= 0x4E00 && r <= 0x9FFF {
			n++
		}
	}
	return n
}
