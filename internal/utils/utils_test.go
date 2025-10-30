package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsAlphaNumericOrSpecialLowercaseLetters Проверяет строчные буквы.
func TestIsAlphaNumericOrSpecialLowercaseLetters(t *testing.T) {
	result := IsAlphaNumericOrSpecial("abcdefghijklmnopqrstuvwxyz")
	assert.True(t, result)
}

// TestIsAlphaNumericOrSpecialUppercaseLetters Проверяет прописные буквы.
func TestIsAlphaNumericOrSpecialUppercaseLetters(t *testing.T) {
	result := IsAlphaNumericOrSpecial("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	assert.True(t, result)
}

// TestIsAlphaNumericOrSpecialNumbers Проверяет цифры.
func TestIsAlphaNumericOrSpecialNumbers(t *testing.T) {
	result := IsAlphaNumericOrSpecial("0123456789")
	assert.True(t, result)
}

// TestIsAlphaNumericOrSpecialAllowedSpecialChars Проверяет разрешённые спецсимволы.
func TestIsAlphaNumericOrSpecialAllowedSpecialChars(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"восклицательный знак", "!", true},
		{"собака", "@", true},
		{"хеш", "#", true},
		{"доллар", "$", true},
		{"процент", "%", true},
		{"крышка", "^", true},
		{"амперсанд", "&", true},
		{"звёздочка", "*", true},
		{"круглая скобка открывающая", "(", true},
		{"круглая скобка закрывающая", ")", true},
		{"подчёркивание", "_", true},
		{"плюс", "+", true},
		{"минус", "-", true},
		{"равно", "=", true},
		{"квадратная скобка открывающая", "[", true},
		{"квадратная скобка закрывающая", "]", true},
		{"фигурная скобка открывающая", "{", true},
		{"фигурная скобка закрывающая", "}", true},
		{"труба", "|", true},
		{"точка с запятой", ";", true},
		{"двоеточие", ":", true},
		{"одинарная кавычка", "'", true},
		{"двойная кавычка", "\"", true},
		{"запятая", ",", true},
		{"точка", ".", true},
		{"меньше", "<", true},
		{"больше", ">", true},
		{"вопрос", "?", true},
		{"слеш", "/", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAlphaNumericOrSpecial(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// TestIsAlphaNumericOrSpecialMixed Проверяет смешанный ввод.
func TestIsAlphaNumericOrSpecialMixed(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"буквы и цифры", "abc123", true},
		{"буквы и спецсимволы", "abc@def", true},
		{"цифры и спецсимволы", "123!456", true},
		{"все вместе", "abc123!@#", true},
		{"с пробелом", "abc def", false},
		{"с кириллицей", "абвгд", false},
		{"с юникодом", "😀", false},
		{"с табуляцией", "abc\tdef", false},
		{"с новой строкой", "abc\ndef", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAlphaNumericOrSpecial(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// TestIsAlphaNumericOrSpecialEmptyString Проверяет пустую строку.
func TestIsAlphaNumericOrSpecialEmptyString(t *testing.T) {
	result := IsAlphaNumericOrSpecial("")
	assert.False(t, result)
}

// TestIsAlphaNumericOrSpecialOnlySpaces Проверяет только пробелы.
func TestIsAlphaNumericOrSpecialOnlySpaces(t *testing.T) {
	result := IsAlphaNumericOrSpecial("   ")
	assert.False(t, result)
}

// TestIsAlphaNumericOrSpecialWithCyrillics Проверяет кириллицу.
func TestIsAlphaNumericOrSpecialWithCyrillics(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"русские буквы", "абвгд", false},
		{"русские буквы с цифрами", "абвгд123", false},
		{"смешанные буквы", "abcабв", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAlphaNumericOrSpecial(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// TestIsAlphaNumericOrSpecialWithWhitespace Проверяет пробельные символы.
func TestIsAlphaNumericOrSpecialWithWhitespace(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"с пробелом", "abc def", false},
		{"с табуляцией", "abc\tdef", false},
		{"с новой строкой", "abc\ndef", false},
		{"с возвратом каретки", "abc\rdef", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAlphaNumericOrSpecial(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// TestIsAlphaNumericOrSpecialSpecialNotAllowed Проверяет недопустимые спецсимволы.
func TestIsAlphaNumericOrSpecialSpecialNotAllowed(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"тильда", "~", false},
		{"обратный апостроф", "`", false},
		{"обратный слеш", "\\", false},
		{"кастрюля", "§", false},
		{"копирайт", "©", false},
		{"неразрывный пробел", "\u00A0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAlphaNumericOrSpecial(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// TestIsAlphaNumericOrSpecialPassword Проверяет валидные пароли.
func TestIsAlphaNumericOrSpecialPassword(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"простой пароль", "Password123", true},
		{"с спецсимволами", "P@ssw0rd!", true},
		{"со всеми спецсимволами", "!@#$%^&*()", true},
		{"только цифры", "12345", true},
		{"с пробелом невалиден", "Pass word", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAlphaNumericOrSpecial(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// TestIsAlphaNumericOrSpecialEdgeCases Проверяет граничные случаи.
func TestIsAlphaNumericOrSpecialEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"одна буква a", "a", true},
		{"одна буква Z", "Z", true},
		{"одна цифра 0", "0", true},
		{"один спецсимвол !", "!", true},
		{"очень длинная строка", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAlphaNumericOrSpecial(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// TestIsAlphaNumericOrSpecialConsecutiveSpecials Проверяет последовательные спецсимволы.
func TestIsAlphaNumericOrSpecialConsecutiveSpecials(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		{"два спецсимвола", "!@", true},
		{"три спецсимвола", "!@#", true},
		{"буквы со спецсимволами", "a!b@c#", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAlphaNumericOrSpecial(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}
