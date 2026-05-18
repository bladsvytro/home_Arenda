package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

// generateID генерирует уникальный ID для дома
func generateID() string {
	return "h" + strconv.FormatInt(time.Now().UnixNano(), 10) + strconv.Itoa(rand.Intn(1000))
}

// initRand инициализирует генератор случайных чисел
func initRand() {
	rand.Seed(time.Now().UnixNano())
}

// parseDate разбирает строку даты в формате YYYY-MM-DD
func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// formatDate форматирует time.Time в YYYY-MM-DD
func formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// monthRange возвращает первую и последнюю дату месяца
func monthRange(year, month int) (first, last string) {
	first = fmt.Sprintf("%04d-%02d-01", year, month)
	lastDay := daysInMonth(year, month)
	last = fmt.Sprintf("%04d-%02d-%02d", year, month, lastDay)
	return
}

// daysInMonth возвращает количество дней в месяце (дублируется из handlers, но для удобства)
func daysInMonth(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// contains проверяет наличие строки в слайсе
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}