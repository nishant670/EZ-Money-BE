package models

import "time"

type SplitFriend struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Name      string    `gorm:"not null" json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Archived  bool      `gorm:"not null;default:false" json:"archived"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SplitBill struct {
	ID           uint               `gorm:"primaryKey" json:"id"`
	UserID       uint               `gorm:"index;not null" json:"user_id"`
	EntryID      *uint              `gorm:"index" json:"entry_id"`
	Entry        *Entry             `json:"entry,omitempty" gorm:"foreignKey:EntryID;constraint:OnDelete:SET NULL"`
	Title        string             `gorm:"not null" json:"title"`
	TotalAmount  Money              `gorm:"type:numeric(19,2);not null" json:"total_amount"`
	Currency     string             `gorm:"type:char(3);not null;default:INR" json:"currency"`
	Date         string             `gorm:"not null" json:"date"`
	Notes        string             `json:"notes"`
	Participants []SplitParticipant `json:"participants,omitempty" gorm:"foreignKey:BillID"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

type SplitParticipant struct {
	ID          uint        `gorm:"primaryKey" json:"id"`
	UserID      uint        `gorm:"index;not null" json:"user_id"`
	BillID      uint        `gorm:"index;not null" json:"bill_id"`
	Bill        SplitBill   `json:"-" gorm:"foreignKey:BillID;constraint:OnDelete:CASCADE"`
	FriendID    uint        `gorm:"index;not null" json:"friend_id"`
	Friend      SplitFriend `json:"friend,omitempty" gorm:"foreignKey:FriendID;constraint:OnDelete:CASCADE"`
	ShareAmount Money       `gorm:"type:numeric(19,2);not null" json:"share_amount"`
	Direction   string      `gorm:"type:varchar(24);not null" json:"direction"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type SplitSettlement struct {
	ID        uint        `gorm:"primaryKey" json:"id"`
	UserID    uint        `gorm:"index;not null" json:"user_id"`
	FriendID  uint        `gorm:"index;not null" json:"friend_id"`
	Friend    SplitFriend `json:"friend,omitempty" gorm:"foreignKey:FriendID;constraint:OnDelete:CASCADE"`
	Amount    Money       `gorm:"type:numeric(19,2);not null" json:"amount"`
	Direction string      `gorm:"type:varchar(24);not null" json:"direction"`
	Date      string      `gorm:"not null" json:"date"`
	Notes     string      `json:"notes"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}
