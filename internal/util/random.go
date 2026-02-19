package util

import (
	"math/rand"
)

// RandomNumbers 生成指定长度的随机数字字符串
func RandomNumbers(n int) string {
	// 定义数字字符集
	digits := "0123456789"
	result := make([]byte, n)

	for i := 0; i < n; i++ {
		// rand.Intn(len(digits)) 生成 0-9 的随机索引
		result[i] = digits[rand.Intn(len(digits))]
	}

	return string(result)
}

// RandomString 生成指定长度的随机小写字符和数字的字符串
func RandomString(n int) string {
	base := "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, n)
	for i := 0; i < n; i++ {
		result[i] = base[rand.Intn(len(base))]
	}
	return string(result)
}
