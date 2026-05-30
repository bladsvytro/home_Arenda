package main

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
)

// adminToken — секрет администратора. По умолчанию «Толкачёва2020».
// Можно переопределить переменной окружения ADMIN_TOKEN.
var adminToken = func() string {
	if t := os.Getenv("ADMIN_TOKEN"); t != "" {
		return t
	}
	return "Толкачёва2020"
}()

// isAdmin проверяет заголовок X-Admin-Token.
// В HTTP-заголовках нельзя передавать не-ASCII символы (кириллицу), поэтому
// фронтенд шлёт токен в base64 (UTF-8). Принимаем оба варианта: и сырой токен
// (для curl/Postman при ASCII-токене), и base64-кодированный.
func isAdmin(r *http.Request) bool {
	got := r.Header.Get("X-Admin-Token")
	if got == "" {
		return false
	}
	want := []byte(adminToken)
	// 1) Прямое совпадение (если токен ASCII и прислан как есть)
	if subtle.ConstantTimeCompare([]byte(got), want) == 1 {
		return true
	}
	// 2) Совпадение после декодирования base64 (кириллический токен из браузера)
	if dec, err := base64.StdEncoding.DecodeString(got); err == nil {
		if subtle.ConstantTimeCompare(dec, want) == 1 {
			return true
		}
	}
	return false
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