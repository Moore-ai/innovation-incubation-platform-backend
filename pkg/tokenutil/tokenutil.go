package tokenutil

import (
	"strings"
	"unicode"
)

// ApproxTokenLen 近似估计 Token 长度，支持中英文混合。
// CJK 字符按 1 token/字，非 CJK 按空白分词计数。
func ApproxTokenLen(text string) int {
	cjk := 0
	for _, r := range text {
		if IsCJK(r) {
			cjk++
		}
	}
	// 过滤纯 CJK 的"词"，避免同一段 CJK 文字被既按字数又按词数重复计算
	nonCJK := 0
	for _, word := range strings.Fields(text) {
		hasCJK := false
		for _, r := range word {
			if IsCJK(r) {
				hasCJK = true
				break
			}
		}
		if !hasCJK {
			nonCJK++
		}
	}
	return cjk + nonCJK
}

// IsCJK 判断 rune 是否为中日韩统一表意文字（CJK Unified Ideographs）
// 覆盖 Unicode 区块：CJK 统一表意文字、扩展 A、扩展 B、兼容表意文字
func IsCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		0x3400 <= r && r <= 0x4DBF ||
		0x20000 <= r && r <= 0x2A6DF ||
		0xF900 <= r && r <= 0xFAFF
}
