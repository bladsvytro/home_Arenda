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

	// Регистрируем обработчики напрямую в http.DefaultServeMux
	http.HandleFunc("/api/houses", func(w http.ResponseWriter, r *http.Request) {
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
	})

	http.HandleFunc("/api/houses/", func(w http.ResponseWriter, r *http.Request) {
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
	})

	http.HandleFunc("/api/upload/", handlers.UploadPhotoHandler)

	http.HandleFunc("/api/calendar/", func(w http.ResponseWriter, r *http.Request) {
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
	})

	http.HandleFunc("/api/calendar_range/", handlers.GetCalendarRangeHandler)

	// Отладочный эндпоинт
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	// Раздаём загруженные фотографии
	os.MkdirAll("uploads", 0755)
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	// Статический фронтенд
	fs := http.FileServer(http.Dir("."))
	http.Handle("/", fs)

	// Переименуем rental.html в index.html для удобства
	http.HandleFunc("/index.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "rental.html")
	})

	port := ":8080"
	log.Printf("Сервер запущен на порту %s", port)
	log.Printf("Доступные эндпоинты:")
	log.Printf("  GET  /api/houses")
	log.Printf("  GET  /api/houses/{id}")
	log.Printf("  GET  /api/calendar/{house_id}?month=YYYY-MM")
	log.Printf("  GET  /api/calendar_range/{house_id}?from=YYYY-MM-DD&to=YYYY-MM-DD")
	log.Printf("  POST /api/houses (требует X-Admin-Token: secret)")
	log.Printf("  PUT  /api/calendar/{house_id} (требует X-Admin-Token: secret)")
	log.Printf("  DELETE /api/houses/{id} (требует X-Admin-Token: secret)")
	log.Printf("Фронтенд доступен по адресу http://localhost%s/", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}