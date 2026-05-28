package main

import (
	"encoding/json"
	"fmt"
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

	// Загружаем демо-данные, если файлы пусты
	if err := loadDemoData(storage); err != nil {
		log.Printf("Предупреждение: не удалось загрузить демо-данные: %v", err)
	}

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

// loadDemoData загружает демо-данные, если файлы пусты
func loadDemoData(storage *Storage) error {
	houses := storage.GetAllHouses()
	if len(houses) > 0 {
		// Данные уже есть
		return nil
	}

	// Создаём демо-дома на основе данных из rental.html
	demoHouses := []House{
		{
			ID:          "h1",
			Name:        "Коттедж «Берёзовая роща»",
			Area:        "Подмосковье, Истринский р-н, 140 м²",
			BasePrice:   150,
			Description: "Просторный двухэтажный коттедж в окружении берёзового леса. Тёплый пол, камин, сауна — всё для идеального отдыха. До ближайшего озера пять минут пешком. В вечернее время слышно, как шумит ветер в кронах деревьев.",
			Amenities:   []string{"Wi-Fi", "Сауна", "Камин", "Парковка", "Кухня", "Барбекю", "Детская площадка"},
			Photos: []string{
				"https://images.unsplash.com/photo-1600585154340-be6161a56a0c?w=800&q=80",
				"https://images.unsplash.com/photo-1580587771525-78b9dba3b914?w=800&q=80",
				"https://images.unsplash.com/photo-1568605114967-8130f3a36994?w=800&q=80",
				"https://images.unsplash.com/photo-1570129477492-45c003edd2be?w=800&q=80",
			},
		},
		{
			ID:          "h2",
			Name:        "Дача «Тихая гавань»",
			Area:        "Карелия, Сортавала, 80 м²",
			BasePrice:   120,
			Description: "Уютная дача на берегу Ладожского озера. Кристально чистая вода, скалистые берега и нетронутая природа Карелии. Идеально для рыбалки и медитативного отдыха вдали от городской суеты.",
			Amenities:   []string{"Wi-Fi", "Лодка", "Рыбалка", "Кухня", "Терраса", "Мангал"},
			Photos: []string{
				"https://images.unsplash.com/photo-1449158743715-0a90ebb6d2d8?w=800&q=80",
				"https://images.unsplash.com/photo-1613490493576-7fde63acd811?w=800&q=80",
				"https://images.unsplash.com/photo-1602343168117-bb8ffe3e2e9f?w=800&q=80",
				"https://images.unsplash.com/photo-1510798831971-661eb04b3739?w=800&q=80",
			},
		},
		{
			ID:          "h3",
			Name:        "Шале «Горный ветер»",
			Area:        "Сочи, Красная Поляна, 200 м²",
			BasePrice:   280,
			Description: "Роскошное шале в горном стиле с панорамными видами на хребет. Большая гостиная с высокими потолками, профессиональная кухня и просторная терраса. В 15 минутах езды от горнолыжных трасс «Газпром» и «Роза Хутор».",
			Amenities:   []string{"Wi-Fi", "Джакузи", "Сауна", "Камин", "Парковка", "Кухня", "Терраса", "Тренажёрный зал"},
			Photos: []string{
				"https://images.unsplash.com/photo-1542718610-a1d656d1884c?w=800&q=80",
				"https://images.unsplash.com/photo-1506905925346-21bda4d32df4?w=800&q=80",
				"https://images.unsplash.com/photo-1530122037265-a5f1f91d3b99?w=800&q=80",
				"https://images.unsplash.com/photo-1520250497591-112f2f40a3f4?w=800&q=80",
			},
		},
	}

	// Добавляем дома в хранилище
	for _, house := range demoHouses {
		if err := storage.AddHouse(house); err != nil {
			return fmt.Errorf("не удалось добавить демо-дом %s: %w", house.ID, err)
		}
	}

	// Создаём демо-календари
	if err := seedDemoCalendars(storage); err != nil {
		return fmt.Errorf("не удалось создать демо-календари: %w", err)
	}

	log.Println("Демо-данные успешно загружены")
	return nil
}

// seedDemoCalendars заполняет календари демо-данными
func seedDemoCalendars(storage *Storage) error {
	// Календарь для дома h1 (октябрь 2026)
	cal1 := make(map[string]CalendarEntry)
	for d := 1; d <= 31; d++ {
		date := fmt.Sprintf("2026-09-%02d", d) // октябрь месяц 09 (0-index)
		busy := (d >= 5 && d <= 10) || (d >= 15 && d <= 20)
		price := 120 + (d*7)%61
		cal1[date] = CalendarEntry{
			Date:     date,
			Price:    price,
			IsBooked: busy,
		}
	}
	// ноябрь
	for d := 1; d <= 30; d++ {
		date := fmt.Sprintf("2026-10-%02d", d)
		price := 130 + (d*11)%51
		cal1[date] = CalendarEntry{
			Date:     date,
			Price:    price,
			IsBooked: false,
		}
	}
	// Сохраняем
	for _, entry := range cal1 {
		if err := storage.UpdateCalendarEntry("h1", entry); err != nil {
			return err
		}
	}

	// Календарь для дома h2
	cal2 := make(map[string]CalendarEntry)
	for d := 1; d <= 31; d++ {
		date := fmt.Sprintf("2026-09-%02d", d)
		price := 100 + (d*9)%41
		cal2[date] = CalendarEntry{
			Date:     date,
			Price:    price,
			IsBooked: false,
		}
	}
	for d := 1; d <= 30; d++ {
		date := fmt.Sprintf("2026-10-%02d", d)
		busy := d >= 12 && d <= 14
		price := 100 + (d*13)%51
		cal2[date] = CalendarEntry{
			Date:     date,
			Price:    price,
			IsBooked: busy,
		}
	}
	for _, entry := range cal2 {
		if err := storage.UpdateCalendarEntry("h2", entry); err != nil {
			return err
		}
	}

	return nil
}