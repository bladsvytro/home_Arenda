package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
)

// adminToken читается из переменной окружения ADMIN_TOKEN.
// Если переменная не задана — используется значение по умолчанию (для локальной разработки).
// На проде обязательно задайте ADMIN_TOKEN, чтобы токен не лежал в коде/гите.
var adminToken = func() string {
	if t := os.Getenv("ADMIN_TOKEN"); t != "" {
		return t
	}
	return "TolkachevaAdmin2020" // запасной токен для локального запуска
}()

// isAdmin проверяет заголовок X-Admin-Token (сравнение в постоянном времени).
func isAdmin(r *http.Request) bool {
	token := r.Header.Get("X-Admin-Token")
	return subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) == 1
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