package main

import (
	"encoding/json"
	"net/http"
)

const adminToken = "TolkachevaAdmin2020" // ASCII-токен для HTTP-заголовков

// isAdmin проверяет заголовок X-Admin-Token
func isAdmin(r *http.Request) bool {
	token := r.Header.Get("X-Admin-Token")
	return token == adminToken
}

// adminMiddleware проверяет авторизацию для методов, требующих прав администратора
func adminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAdmin(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(Response{
				Success: false,
				Error:   "требуется авторизация администратора",
			})
			return
		}
		next(w, r)
	}
}

// corsMiddleware добавляет заголовки CORS для разработки
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Разрешаем любые источники
		w.Header().Set("Access-Control-Allow-Origin", "*")
		// Разрешаем необходимые методы
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		// Разрешаем необходимые заголовки
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Admin-Token")

		// Обработка предварительного запроса OPTIONS
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// jsonMiddleware устанавливает Content-Type application/json для ответов
func jsonMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next(w, r)
	}
}

// logMiddleware логирует запросы
func logMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// В реальном приложении можно добавить логгер
		// log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL.Path)
		next(w, r)
	}
}

// chainMiddleware применяет несколько middleware к обработчику
func chainMiddleware(h http.HandlerFunc, middlewares ...func(http.HandlerFunc) http.HandlerFunc) http.HandlerFunc {
	for _, m := range middlewares {
		h = m(h)
	}
	return h
}