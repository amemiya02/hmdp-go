package util

import (
	"regexp"

	"github.com/amemiya02/hmdp-go/internal/constant"
)

// IsPhoneInvalid 校验手机号是否无效（无效返回true）
func IsPhoneInvalid(phone string) bool {
	reg := regexp.MustCompile(constant.PHONE_REGEX)
	return !reg.MatchString(phone)
}

// IsVerifyCodeInvalid 校验验证码是否无效（6位数字/字母）
func IsVerifyCodeInvalid(code string) bool {
	reg := regexp.MustCompile(constant.VERIFY_CODE_REGEX)
	return !reg.MatchString(code)
}
