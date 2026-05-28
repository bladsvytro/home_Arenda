package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Storage управляет данными в JSON файлах
type Storage struct {
	housesPath   string
	calendarDir  string
	houses       []House
	housesMutex  sync.RWMutex
	calendarMutex sync.Map // для каждого house_id свой sync.RWMutex
}

// NewStorage создаёт новый Storage и загружает данные
func NewStorage(dataDir string) (*Storage, error) {
	housesPath := filepath.Join(dataDir, "houses.json")
	calendarDir := filepath.Join(dataDir, "calendar")

	// Создаём директории, если их нет
	if err := os.MkdirAll(calendarDir, 0755); err != nil {
		return nil, fmt.Errorf("не удалось создать директорию календарей: %w", err)
	}

	s := &Storage{
		housesPath:  housesPath,
		calendarDir: calendarDir,
	}

	// Загружаем дома
	if err := s.loadHouses(); err != nil {
		return nil, fmt.Errorf("не удалось загрузить дома: %w", err)
	}

	return s, nil
}

// loadHouses читает houses.json в память
func (s *Storage) loadHouses() error {
	s.housesMutex.Lock()
	defer s.housesMutex.Unlock()

	data, err := os.ReadFile(s.housesPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Файла нет — создаём пустой массив
			s.houses = []House{}
			return nil
		}
		return err
	}

	if err := json.Unmarshal(data, &s.houses); err != nil {
		return fmt.Errorf("ошибка парсинга houses.json: %w", err)
	}
	return nil
}

// saveHouses записывает дома обратно в файл
func (s *Storage) saveHouses() error {
	s.housesMutex.RLock()
	data, err := json.MarshalIndent(s.houses, "", "  ")
	s.housesMutex.RUnlock()
	if err != nil {
		return err
	}

	return os.WriteFile(s.housesPath, data, 0644)
}

// GetAllHouses возвращает все дома
func (s *Storage) GetAllHouses() []House {
	s.housesMutex.RLock()
	defer s.housesMutex.RUnlock()
	// Возвращаем копию, чтобы избежать гонок
	houses := make([]House, len(s.houses))
	copy(houses, s.houses)
	return houses
}

// GetHouse возвращает дом по ID
func (s *Storage) GetHouse(id string) (*House, error) {
	s.housesMutex.RLock()
	defer s.housesMutex.RUnlock()

	for _, h := range s.houses {
		if h.ID == id {
			// Возвращаем копию
			houseCopy := h
			return &houseCopy, nil
		}
	}
	return nil, fmt.Errorf("дом с ID %s не найден", id)
}

// AddHouse добавляет новый дом и сохраняет
func (s *Storage) AddHouse(house House) error {
	s.housesMutex.Lock()
	s.houses = append(s.houses, house)
	s.housesMutex.Unlock()

	return s.saveHouses()
}

// UpdateHouse обновляет существующий дом
func (s *Storage) UpdateHouse(id string, updated House) error {
	s.housesMutex.Lock()
	found := false
	for i, h := range s.houses {
		if h.ID == id {
			s.houses[i] = updated
			found = true
			break
		}
	}
	s.housesMutex.Unlock() // разблокируем ДО записи в файл

	if !found {
		return fmt.Errorf("дом с ID %s не найден", id)
	}
	return s.saveHouses()
}

// DeleteHouse удаляет дом по ID
func (s *Storage) DeleteHouse(id string) error {
	s.housesMutex.Lock()
	found := false
	for i, h := range s.houses {
		if h.ID == id {
			s.houses = append(s.houses[:i], s.houses[i+1:]...)
			found = true
			break
		}
	}
	s.housesMutex.Unlock() // разблокируем ДО записи в файл

	if !found {
		return fmt.Errorf("дом с ID %s не найден", id)
	}
	return s.saveHouses()
}

// calendarPath возвращает путь к файлу календаря для houseID
func (s *Storage) calendarPath(houseID string) string {
	return filepath.Join(s.calendarDir, fmt.Sprintf("calendar_%s.json", houseID))
}

// LoadCalendar загружает календарь для дома
func (s *Storage) LoadCalendar(houseID string) (map[string]CalendarEntry, error) {
	path := s.calendarPath(houseID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Файла нет — возвращаем пустой календарь
			return make(map[string]CalendarEntry), nil
		}
		return nil, err
	}

	var calendar map[string]CalendarEntry
	if err := json.Unmarshal(data, &calendar); err != nil {
		return nil, fmt.Errorf("ошибка парсинга календаря для %s: %w", houseID, err)
	}
	return calendar, nil
}

// SaveCalendar сохраняет календарь для дома
func (s *Storage) SaveCalendar(houseID string, calendar map[string]CalendarEntry) error {
	path := s.calendarPath(houseID)
	data, err := json.MarshalIndent(calendar, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// UpdateCalendarEntry обновляет или добавляет запись в календаре
func (s *Storage) UpdateCalendarEntry(houseID string, entry CalendarEntry) error {
	// Получаем мьютекс для этого houseID
	mutex, _ := s.calendarMutex.LoadOrStore(houseID, &sync.RWMutex{})
	mu := mutex.(*sync.RWMutex)
	mu.Lock()
	defer mu.Unlock()

	calendar, err := s.LoadCalendar(houseID)
	if err != nil {
		return err
	}

	calendar[entry.Date] = entry
	return s.SaveCalendar(houseID, calendar)
}

// GetCalendarRange возвращает записи календаря за диапазон дат
func (s *Storage) GetCalendarRange(houseID, from, to string) ([]CalendarEntry, error) {
	calendar, err := s.LoadCalendar(houseID)
	if err != nil {
		return nil, err
	}

	var result []CalendarEntry
	for date, entry := range calendar {
		if date >= from && date <= to {
			result = append(result, entry)
		}
	}
	return result, nil
}