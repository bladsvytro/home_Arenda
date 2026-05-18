package main

// House представляет дом/коттедж для аренды
type House struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Area        string   `json:"area"`
	BasePrice   int      `json:"base_price"`
	Photos      []string `json:"photos"`
	Description string   `json:"description"`
	Amenities   []string `json:"amenities"`
}

// CalendarEntry представляет запись в календаре для конкретной даты
type CalendarEntry struct {
	Date     string `json:"date"`      // YYYY-MM-DD
	Price    int    `json:"price"`     // цена на эту дату
	IsBooked bool   `json:"is_booked"` // забронирована ли дата
}

// CalendarUpdate представляет обновление одной даты в календаре
type CalendarUpdate struct {
	Date     string `json:"date"`
	Price    int    `json:"price"`
	IsBooked bool   `json:"is_booked"`
}

// HouseCreate представляет данные для создания нового дома
type HouseCreate struct {
	Name        string   `json:"name"`
	Area        string   `json:"area"`
	BasePrice   int      `json:"base_price"`
	Photos      []string `json:"photos"`
	Description string   `json:"description"`
	Amenities   []string `json:"amenities"`
}

// ValidationError представляет ошибку валидации
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// Response общий ответ API
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}