package agent

import "innovation-incubation-platform-backend/pkg/tokenutil"

// ApproxTokenLen 近似估计 Token 长度，委托给 pkg/tokenutil。
func ApproxTokenLen(text string) int {
	return tokenutil.ApproxTokenLen(text)
}

// IsCJK 判断 rune 是否为中日韩统一表意文字，委托给 pkg/tokenutil。
func IsCJK(r rune) bool {
	return tokenutil.IsCJK(r)
}
