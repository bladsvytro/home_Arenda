package main

import "time"

// daysInMonth возвращает количество дней в месяце
func daysInMonth(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}