package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Handlers содержит зависимости (storage) для обработчиков
type Handlers struct {
	storage *Storage
}

// NewHandlers создаёт новый экземпляр Handlers
func NewHandlers(storage *Storage) *Handlers {
	return &Handlers{storage: storage}
}

// writeJSON записывает JSON ответ
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError записывает ошибку в формате JSON
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, Response{
		Success: false,
		Error:   message,
	})
}

// validateHouseCreate проверяет данные для создания дома
func validateHouseCreate(h HouseCreate) []ValidationError {
	var errors []ValidationError
	if strings.TrimSpace(h.Name) == "" {
		errors = append(errors, ValidationError{Field: "name", Message: "название обязательно"})
	}
	if h.BasePrice <= 0 {
		errors = append(errors, ValidationError{Field: "base_price", Message: "цена должна быть положительной"})
	}
	if len(h.Photos) == 0 {
		errors = append(errors, ValidationError{Field: "photos", Message: "должна быть хотя бы одна фотография"})
	}
	return errors
}

// GetAllHousesHandler возвращает все дома
func (h *Handlers) GetAllHousesHandler(w http.ResponseWriter, r *http.Request) {
	houses := h.storage.GetAllHouses()
	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    houses,
	})
}

// GetHouseHandler возвращает один дом по ID
func (h *Handlers) GetHouseHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/houses/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "ID дома не указан")
		return
	}

	house, err := h.storage.GetHouse(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    house,
	})
}

// CreateHouseHandler создаёт новый дом
func (h *Handlers) CreateHouseHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем авторизацию (админ)
	if !isAdmin(r) {
		writeError(w, http.StatusUnauthorized, "требуется авторизация администратора")
		return
	}

	var input HouseCreate
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "неверный формат JSON")
		return
	}

	// Валидация
	if errs := validateHouseCreate(input); len(errs) > 0 {
		writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "ошибки валидации",
			Data:    errs,
		})
		return
	}

	// Генерируем ID (простой способ)
	id := fmt.Sprintf("h%d", time.Now().UnixNano())

	// Перемещаем temp-фото в постоянную директорию дома
	photos, err := h.storage.MovePhotosFromTemp(id, input.Photos)
	if err != nil {
		log.Printf("Предупреждение: не удалось переместить фото: %v", err)
		photos = input.Photos
	}

	house := House{
		ID:          id,
		Name:        input.Name,
		Area:        input.Area,
		BasePrice:   input.BasePrice,
		Photos:      photos,
		Description: input.Description,
		Amenities:   input.Amenities,
	}

	if err := h.storage.AddHouse(house); err != nil {
		writeError(w, http.StatusInternalServerError, "не удалось сохранить дом")
		return
	}

	writeJSON(w, http.StatusCreated, Response{
		Success: true,
		Data:    house,
	})
}

// UpdateHouseHandler обновляет дом по ID
func (h *Handlers) UpdateHouseHandler(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		writeError(w, http.StatusUnauthorized, "требуется авторизация администратора")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/houses/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "ID дома не указан")
		return
	}

	var input HouseCreate
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "неверный формат JSON")
		return
	}

	if errs := validateHouseCreate(input); len(errs) > 0 {
		writeJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "ошибки валидации",
			Data:    errs,
		})
		return
	}

	// Перемещаем temp-фото в постоянную директорию дома (если есть новые)
	photos, err := h.storage.MovePhotosFromTemp(id, input.Photos)
	if err != nil {
		log.Printf("Предупреждение: не удалось переместить фото при обновлении: %v", err)
		photos = input.Photos
	}

	updated := House{
		ID:          id,
		Name:        input.Name,
		Area:        input.Area,
		BasePrice:   input.BasePrice,
		Photos:      photos,
		Description: input.Description,
		Amenities:   input.Amenities,
	}

	if err := h.storage.UpdateHouse(id, updated); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    updated,
	})
}

// UploadPhotoHandler принимает файл и сохраняет в uploads/{houseId}/
func (h *Handlers) UploadPhotoHandler(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		writeError(w, http.StatusUnauthorized, "требуется авторизация администратора")
		return
	}

	houseID := strings.TrimPrefix(r.URL.Path, "/api/upload/")
	if houseID == "" {
		houseID = "temp"
	}
	// Sanitize houseID
	houseID = strings.ReplaceAll(houseID, "..", "")
	houseID = strings.ReplaceAll(houseID, "/", "")

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "ошибка парсинга формы: "+err.Error())
		return
	}

	file, header, err := r.FormFile("photo")
	if err != nil {
		writeError(w, http.StatusBadRequest, "файл не найден в запросе")
		return
	}
	defer file.Close()

	// Проверяем расширение
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true}
	if !allowed[ext] {
		writeError(w, http.StatusBadRequest, "допустимые форматы: jpg, png, webp, gif")
		return
	}

	dir := filepath.Join("uploads", houseID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "не удалось создать директорию")
		return
	}

	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	dstPath := filepath.Join(dir, filename)

	dst, err := os.Create(dstPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "не удалось создать файл")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		writeError(w, http.StatusInternalServerError, "ошибка записи файла")
		return
	}

	url := fmt.Sprintf("/uploads/%s/%s", houseID, filename)
	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    map[string]string{"url": url},
	})
}

// DeleteHouseHandler удаляет дом
func (h *Handlers) DeleteHouseHandler(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		writeError(w, http.StatusUnauthorized, "требуется авторизация администратора")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/houses/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "ID дома не указан")
		return
	}

	if err := h.storage.DeleteHouse(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Удаляем фото и календарь дома асинхронно.
	// Запускаем после ответа, чтобы не задерживать клиента.
	// gitCommitAndPush внутри подберёт все изменения разом.
	go h.storage.DeleteHouseFiles(id)

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    map[string]string{"message": "дом удалён"},
	})
}

// GetCalendarHandler возвращает календарь на указанный месяц
func (h *Handlers) GetCalendarHandler(w http.ResponseWriter, r *http.Request) {
	// Извлекаем house_id из пути (например, /api/calendar/{house_id})
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		writeError(w, http.StatusBadRequest, "неверный путь")
		return
	}
	houseID := pathParts[3]

	month := r.URL.Query().Get("month")
	if month == "" {
		writeError(w, http.StatusBadRequest, "параметр month обязателен")
		return
	}

	// Проверяем формат месяца YYYY-MM
	parts := strings.Split(month, "-")
	if len(parts) != 2 {
		writeError(w, http.StatusBadRequest, "месяц должен быть в формате YYYY-MM")
		return
	}
	year, err1 := strconv.Atoi(parts[0])
	monthNum, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || monthNum < 1 || monthNum > 12 {
		writeError(w, http.StatusBadRequest, "неверный формат месяца")
		return
	}

	// Определяем диапазон дат месяца
	firstDay := fmt.Sprintf("%s-01", month)
	lastDay := fmt.Sprintf("%s-%02d", month, daysInMonth(year, monthNum))

	entries, err := h.storage.GetCalendarRange(houseID, firstDay, lastDay)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "не удалось загрузить календарь")
		return
	}

	// Если записей нет, возвращаем пустой массив
	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    entries,
	})
}

// UpdateCalendarHandler обновляет одну дату в календаре
func (h *Handlers) UpdateCalendarHandler(w http.ResponseWriter, r *http.Request) {
	if !isAdmin(r) {
		writeError(w, http.StatusUnauthorized, "требуется авторизация администратора")
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		writeError(w, http.StatusBadRequest, "неверный путь")
		return
	}
	houseID := pathParts[3]

	var update CalendarUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeError(w, http.StatusBadRequest, "неверный формат JSON")
		return
	}

	// Проверяем формат даты
	if _, err := time.Parse("2006-01-02", update.Date); err != nil {
		writeError(w, http.StatusBadRequest, "дата должна быть в формате YYYY-MM-DD")
		return
	}

	// Для демо пропускаем проверку на будущую дату
	// if update.Date < time.Now().Format("2006-01-02") {
	//     writeError(w, http.StatusBadRequest, "можно обновлять только будущие даты")
	//     return
	// }

	entry := CalendarEntry{
		Date:     update.Date,
		Price:    update.Price,
		IsBooked: update.IsBooked,
	}

	if err := h.storage.UpdateCalendarEntry(houseID, entry); err != nil {
		writeError(w, http.StatusInternalServerError, "не удалось обновить календарь")
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    entry,
	})
}

// GetCalendarRangeHandler возвращает календарь на диапазон дат
func (h *Handlers) GetCalendarRangeHandler(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		writeError(w, http.StatusBadRequest, "неверный путь")
		return
	}
	houseID := pathParts[3]

	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		writeError(w, http.StatusBadRequest, "параметры from и to обязательны")
		return
	}

	// Проверяем формат дат
	if _, err := time.Parse("2006-01-02", from); err != nil {
		writeError(w, http.StatusBadRequest, "from должен быть в формате YYYY-MM-DD")
		return
	}
	if _, err := time.Parse("2006-01-02", to); err != nil {
		writeError(w, http.StatusBadRequest, "to должен быть в формате YYYY-MM-DD")
		return
	}

	entries, err := h.storage.GetCalendarRange(houseID, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "не удалось загрузить календарь")
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    entries,
	})
}
