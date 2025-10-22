package utils

import (
	"strings"
)

// IsAlphaNumericOrSpecial Проверяет что в строке только большие и маленькие буквы английского алфавита, цифры и разрешённые спецсимволы.
func IsAlphaNumericOrSpecial(s string) bool {
	if len(s) == 0 {
		return false
	}

	allowedSpecial := "!@#$%^&*()_+-=[]{}|;:'\",.<>?/"

	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		if strings.ContainsRune(allowedSpecial, r) {
			continue
		}
		return false
	}

	return true
}
