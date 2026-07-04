package tokenutil

import (
	"strings"
	"unicode"
)

// EstimationMode 决定使用哪种估算算法，默认 "better"。
// 应在服务启动时通过 SetEstimationMode 设置一次。
var estimationMode = "better"

// SetEstimationMode 设置 Token 估算模式: "simple" 或 "better"。
func SetEstimationMode(mode string) {
	if mode == "better" {
		estimationMode = "better"
	} else {
		estimationMode = "simple"
	}
}

// Estimate 根据当前估算模式返回近似 Token 长度。
func Estimate(text string) int {
	if estimationMode == "simple" {
		return simpleEstimate(text)
	}
	return betterEstimate(text)
}

// IsCJK 判断 rune 是否为中日韩统一表意文字（CJK Unified Ideographs）
func IsCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		0x3400 <= r && r <= 0x4DBF ||
		0x20000 <= r && r <= 0x2A6DF ||
		0xF900 <= r && r <= 0xFAFF
}

// simpleEstimate CJK 字符按 1 token/字，非 CJK 按空白分词计数。
func simpleEstimate(text string) int {
	cjk := 0
	for _, r := range text {
		if IsCJK(r) {
			cjk++
		}
	}
	nonCJK := 0
	for word := range strings.FieldsSeq(text) {
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

// betterEstimate 基于字符类别估算 Token 长度。
func betterEstimate(text string) int {
	cjk := 0
	ascii := 0
	for _, r := range text {
		if IsCJK(r) {
			cjk++
		} else if r <= 0x7F {
			ascii++
		}
	}
	return int(float64(cjk)*1.6 + float64(ascii)/3.5)
}
