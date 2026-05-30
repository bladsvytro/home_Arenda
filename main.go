package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func main() {
	// Инициализируем хранилище
	storage, err := NewStorage("data")
	if err != nil {
		log.Fatalf("Ошибка инициализации хранилища: %v", err)
	}

	// Инициализируем обработчики
	handlers := NewHandlers(storage)

	// Все API-роуты оборачиваем в corsMiddleware (CORS-заголовки + обработка OPTIONS).
	http.HandleFunc("/api/houses", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.GetAllHousesHandler(w, r)
		case http.MethodPost:
			handlers.CreateHouseHandler(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(Response{
				Success: false,
				Error:   "метод не поддерживается",
			})
		}
	}))

	http.HandleFunc("/api/houses/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.GetHouseHandler(w, r)
		case http.MethodPut:
			handlers.UpdateHouseHandler(w, r)
		case http.MethodDelete:
			handlers.DeleteHouseHandler(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(Response{
				Success: false,
				Error:   "метод не поддерживается",
			})
		}
	}))

	http.HandleFunc("/api/upload/", corsMiddleware(handlers.UploadPhotoHandler))

	http.HandleFunc("/api/calendar/", corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.GetCalendarHandler(w, r)
		case http.MethodPut:
			handlers.UpdateCalendarHandler(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(Response{
				Success: false,
				Error:   "метод не поддерживается",
			})
		}
	}))

	http.HandleFunc("/api/calendar_range/", corsMiddleware(handlers.GetCalendarRangeHandler))

	// Отладочный эндпоинт
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	// Раздаём загруженные фотографии
	os.MkdirAll("uploads", 0755)
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	// Статический фронтенд.
	// Главная страница (/ и /index.html) всегда отдаёт rental.html —
	// единственный источник правды для фронтенда. Остальные пути (css, js,
	// картинки) обслуживает обычный файловый сервер.
	fs := http.FileServer(http.Dir("."))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, "rental.html")
			return
		}
		fs.ServeHTTP(w, r)
	})

	// Порт берётся из переменной окружения PORT (по умолчанию 8080)
	port := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		port = ":" + p
	}
	log.Printf("Сервер запущен на порту %s", port)
	log.Printf("Доступные эндпоинты:")
	log.Printf("  GET  /api/houses")
	log.Printf("  GET  /api/houses/{id}")
	log.Printf("  GET  /api/calendar/{house_id}?month=YYYY-MM")
	log.Printf("  GET  /api/calendar_range/{house_id}?from=YYYY-MM-DD&to=YYYY-MM-DD")
	log.Printf("  POST /api/houses (требует заголовок X-Admin-Token)")
	log.Printf("  PUT  /api/calendar/{house_id} (требует заголовок X-Admin-Token)")
	log.Printf("  DELETE /api/houses/{id} (требует заголовок X-Admin-Token)")
	log.Printf("Admin-токен берётся из переменной окружения ADMIN_TOKEN")
	log.Printf("Фронтенд доступен по адресу http://localhost%s/", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}