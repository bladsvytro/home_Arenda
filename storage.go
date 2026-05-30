package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Storage управляет данными в JSON файлах
type Storage struct {
	housesPath    string
	calendarDir   string
	houses        []House
	housesMutex   sync.RWMutex
	calendarMutex sync.Map // для каждого house_id свой sync.RWMutex
	gitEnabled    bool
	gitRepoDir    string       // корень git-репозитория
	gitSignal     chan struct{} // буферизованный канал — debounce-сигнал для воркера
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
		housesPath: housesPath,
		calendarDir: calendarDir,
		gitSignal:  make(chan struct{}, 1), // буфер 1: если сигнал уже есть — дубль не нужен
	}

	// Загружаем дома
	if err := s.loadHouses(); err != nil {
		return nil, fmt.Errorf("не удалось загрузить дома: %w", err)
	}

	// Проверяем, доступен ли git, и находим корень репозитория
	s.checkGit()

	return s, nil
}

// checkGit проверяет, доступен ли git, находит корень репозитория и запускает воркер.
func (s *Storage) checkGit() {
	startDir := filepath.Dir(s.housesPath) // data/
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = startDir
	out, err := cmd.Output()
	if err != nil {
		log.Printf("Git не доступен (%v), авто-коммиты отключены", err)
		s.gitEnabled = false
		return
	}
	s.gitRepoDir = strings.TrimSpace(string(out))
	s.gitEnabled = true
	log.Printf("Git доступен, корень репозитория: %s", s.gitRepoDir)

	// Запускаем единственный фоновый воркер для git-операций.
	// Все коммиты проходят через него строго по очереди — гонок нет.
	go s.gitWorker()
}

// gitCommitAndPush сигнализирует воркеру о наличии изменений.
// Не блокирует — возврат мгновенный. Несколько вызовов подряд схлопываются в один коммит.
func (s *Storage) gitCommitAndPush() {
	if !s.gitEnabled {
		return
	}
	// Отправляем сигнал в буферизованный канал.
	// Если сигнал уже лежит (буфер заполнен) — молча пропускаем дубль.
	select {
	case s.gitSignal <- struct{}{}:
	default:
	}
}

// gitWorker — единственный горутин, выполняющий git-операции.
// Запускается один раз при старте. Все коммиты идут строго последовательно.
func (s *Storage) gitWorker() {
	for range s.gitSignal {
		// Debounce: ждём 2 секунды тишины, чтобы собрать серию изменений в один коммит.
		timer := time.NewTimer(2 * time.Second)
	drain:
		for {
			select {
			case <-s.gitSignal:
				// Пришёл ещё один сигнал — сбрасываем таймер
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(2 * time.Second)
			case <-timer.C:
				break drain
			}
		}

		s.doCommitAndPush()
	}
}

// doCommitAndPush выполняет git add / commit / push.
// Вызывается только из gitWorker — гарантированно однопоточно.
func (s *Storage) doCommitAndPush() {
	// git add: данные + фотографии (uploads/temp/ игнорируется через .gitignore)
	if err := s.gitExec("add", "-A", "--", "data/", "uploads/"); err != nil {
		log.Printf("Git add error: %v", err)
		return
	}

	// Проверяем, есть ли изменения
	statusCmd := exec.Command("git", "status", "--porcelain", "--", "data/", "uploads/")
	statusCmd.Dir = s.gitRepoDir
	out, _ := statusCmd.Output()
	if len(out) == 0 {
		return // ничего нет — пуш не нужен
	}

	// git commit
	msg := fmt.Sprintf("auto-save: %s", time.Now().Format("2006-01-02 15:04:05"))
	if err := s.gitExec("commit", "-m", msg); err != nil {
		log.Printf("Git commit error: %v", err)
		return
	}

	// git push
	pushCmd := exec.Command("git", "push")
	pushCmd.Dir = s.gitRepoDir
	pushOut, err := pushCmd.CombinedOutput()
	if err != nil {
		log.Printf("Git push error: %v\n%s", err, string(pushOut))
		return
	}
	log.Printf("Git: данные сохранены и отправлены (%s)", strings.TrimSpace(string(pushOut)))
}

// gitExec выполняет git-команду в корне репозитория.
func (s *Storage) gitExec(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = s.gitRepoDir
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// loadHouses читает houses.json в память
func (s *Storage) loadHouses() error {
	s.housesMutex.Lock()
	defer s.housesMutex.Unlock()

	data, err := os.ReadFile(s.housesPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Файла нет — создаём пустой массив и сразу сохраняем
			s.houses = []House{}
			return s.saveHousesLocked()
		}
		return err
	}

	if err := json.Unmarshal(data, &s.houses); err != nil {
		return fmt.Errorf("ошибка парсинга houses.json: %w", err)
	}
	return nil
}

// saveHousesLocked записывает дома в файл (мьютекс уже захвачен)
func (s *Storage) saveHousesLocked() error {
	data, err := json.MarshalIndent(s.houses, "", "  ")
	if err != nil {
		return err
	}

	// Атомарная запись: пишем во временный файл, затем переименовываем
	tmpPath := s.housesPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, s.housesPath); err != nil {
		return err
	}

	// Авто-коммит в git
	s.gitCommitAndPush()

	return nil
}

// saveHouses записывает дома обратно в файл (захватывает мьютекс)
func (s *Storage) saveHouses() error {
	s.housesMutex.RLock()
	defer s.housesMutex.RUnlock()
	return s.saveHousesLocked()
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
	err := s.saveHousesLocked()
	s.housesMutex.Unlock()

	return err
}

// UpdateHouse обновляет существующий дом
func (s *Storage) UpdateHouse(id string, updated House) error {
	s.housesMutex.Lock()
	defer s.housesMutex.Unlock()

	found := false
	for i, h := range s.houses {
		if h.ID == id {
			s.houses[i] = updated
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("дом с ID %s не найден", id)
	}
	return s.saveHousesLocked()
}

// DeleteHouse удаляет дом по ID
func (s *Storage) DeleteHouse(id string) error {
	s.housesMutex.Lock()
	defer s.housesMutex.Unlock()

	found := false
	for i, h := range s.houses {
		if h.ID == id {
			s.houses = append(s.houses[:i], s.houses[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("дом с ID %s не найден", id)
	}
	return s.saveHousesLocked()
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

// SaveCalendar сохраняет календарь для дома (атомарно)
func (s *Storage) SaveCalendar(houseID string, calendar map[string]CalendarEntry) error {
	path := s.calendarPath(houseID)
	data, err := json.MarshalIndent(calendar, "", "  ")
	if err != nil {
		return err
	}

	// Атомарная запись: пишем во временный файл, затем переименовываем
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	// Авто-коммит в git
	s.gitCommitAndPush()

	return nil
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

// MovePhotosFromTemp перемещает фотографии из uploads/temp/ в uploads/{houseID}/.
// Возвращает обновлённые URL (temp-ссылки заменяются на постоянные).
func (s *Storage) MovePhotosFromTemp(houseID string, photos []string) ([]string, error) {
	houseDir := filepath.Join("uploads", houseID)
	if err := os.MkdirAll(houseDir, 0755); err != nil {
		return nil, fmt.Errorf("не удалось создать директорию дома: %w", err)
	}

	updated := make([]string, len(photos))
	for i, photo := range photos {
		// Обрабатываем только локальные temp-фото
		if !strings.HasPrefix(photo, "/uploads/temp/") {
			updated[i] = photo
			continue
		}
		filename := filepath.Base(photo)
		srcPath := filepath.Join("uploads", "temp", filename)
		dstPath := filepath.Join(houseDir, filename)

		// Пробуем переименовать; если не выходит (разные тома) — копируем
		if err := os.Rename(srcPath, dstPath); err != nil {
			if copyErr := copyFile(srcPath, dstPath); copyErr != nil {
				log.Printf("Не удалось переместить фото %s: %v", filename, copyErr)
				updated[i] = photo // оставляем старый URL при ошибке
				continue
			}
			os.Remove(srcPath)
		}
		updated[i] = fmt.Sprintf("/uploads/%s/%s", houseID, filename)
	}
	return updated, nil
}

// copyFile копирует содержимое файла src в dst (используется как fallback для Rename).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// DeleteHouseFiles удаляет фотографии и файл календаря для дома.
// Нужно вызывать ПОСЛЕ удаления дома из JSON (чтобы git-коммит захватил оба изменения).
func (s *Storage) DeleteHouseFiles(houseID string) {
	// Удаляем директорию с фотографиями
	photoDir := filepath.Join("uploads", houseID)
	if err := os.RemoveAll(photoDir); err != nil {
		log.Printf("Не удалось удалить фото дома %s: %v", houseID, err)
	} else {
		log.Printf("Удалена папка с фото дома %s", houseID)
	}

	// Удаляем файл календаря
	calPath := s.calendarPath(houseID)
	if err := os.Remove(calPath); err != nil && !os.IsNotExist(err) {
		log.Printf("Не удалось удалить календарь дома %s: %v", houseID, err)
	}

	// Запускаем коммит: к этому моменту дом уже удалён из houses.json
	// и файлы удалены — git add подберёт все изменения разом
	s.gitCommitAndPush()
}