package main

import (
	"regexp"
	"time"
)

// daysInMonth возвращает количество дней в месяце
func daysInMonth(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// validIDPattern — ID дома состоит только из букв, цифр, дефиса и подчёркивания.
// Это исключает обход каталогов (./ ../ слэши, обратные слэши, нулевые байты и т.п.).
var validIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// sanitizeID проверяет, что идентификатор безопасен для использования в пути файла.
// Возвращает пустую строку, если ID невалиден.
func sanitizeID(id string) string {
	if id == "" || len(id) > 128 || !validIDPattern.MatchString(id) {
		return ""
	}
	return id
}