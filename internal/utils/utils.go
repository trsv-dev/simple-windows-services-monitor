package utils

import (
	"net"
)

// IsAlphaNumeric Проверяет что в строке только большие и маленькие буквы английского алфавита и цифры.
func IsAlphaNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}

	for _, r := range s {
		if (r < 'a' || r > 'z') &&
			(r < 'A' || r > 'Z') &&
			(r < '0' || r > '9') {
			return false
		}
	}

	return true
}

// IsDigitsOnly Проверяет что в строке только цифры.
func IsDigitsOnly(s string) bool {
	if len(s) == 0 {
		return false
	}

	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

// IsValidIP Валидатор IP адресов серверов.
func IsValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}
