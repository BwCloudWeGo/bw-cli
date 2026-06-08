package validator

import "strings"

// Required 判断所有字符串在 trim 后是否都非空。
func Required(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}
