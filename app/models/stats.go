package models

// DailyStats repräsentiert Statistiken für einen einzelnen Tag
type DailyStats struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}
