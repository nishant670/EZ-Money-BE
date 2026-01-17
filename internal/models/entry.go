package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type Entry struct {
	ID          uint        `gorm:"primaryKey" json:"id"`
	Title       string      `json:"title"`
	Type        string      `json:"type"`
	Amount      float64     `json:"amount"`
	Currency    string      `json:"currency"`
	Mode        string      `json:"mode"`
	CardNetwork string      `json:"card_network"`
	Category    string      `json:"category"`
	Merchant    string      `json:"merchant"`
	PurposeType string      `json:"purpose_type"`
	Tag         string      `json:"tag"`
	Tags        StringArray `gorm:"type:jsonb" json:"tags"`
	Notes       string      `json:"notes"`
	Date        string      `json:"date"`
	Time        string      `json:"time"`
	SourceText  string      `json:"source_text"`
	Attachment  string      `json:"attachment"`

	UserID uint `json:"user_id"`
	User   User `json:"-" gorm:"foreignKey:UserID"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type StringArray []string

func (sa StringArray) Value() (driver.Value, error) {
	if len(sa) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(sa)
}

func (sa *StringArray) Scan(value interface{}) error {
	if value == nil {
		*sa = nil
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("unsupported type for StringArray: %T", value)
	}
	if len(data) == 0 {
		*sa = nil
		return nil
	}
	return json.Unmarshal(data, sa)
}
