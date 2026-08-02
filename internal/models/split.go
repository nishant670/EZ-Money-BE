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

type SplitGroup struct {
	ID                  uint               `gorm:"primaryKey" json:"id"`
	UserID              uint               `gorm:"index;not null" json:"user_id"`
	Name                string             `gorm:"not null" json:"name"`
	Archived            bool               `gorm:"not null;default:false" json:"archived"`
	Members             []SplitGroupMember `json:"members,omitempty" gorm:"foreignKey:GroupID"`
	ViewerRole          string             `gorm:"-" json:"viewer_role,omitempty"`
	ViewerCanAddExpense bool               `gorm:"-" json:"viewer_can_add_expense,omitempty"`
	ViewerCanManage     bool               `gorm:"-" json:"viewer_can_manage,omitempty"`
	CreatedAt           time.Time          `json:"created_at"`
	UpdatedAt           time.Time          `json:"updated_at"`
}

type SplitGroupMember struct {
	ID        uint        `gorm:"primaryKey" json:"id"`
	UserID    uint        `gorm:"index;not null" json:"user_id"`
	GroupID   uint        `gorm:"index;not null" json:"group_id"`
	Group     SplitGroup  `json:"-" gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE"`
	FriendID  uint        `gorm:"index;not null" json:"friend_id"`
	Friend    SplitFriend `json:"friend,omitempty" gorm:"foreignKey:FriendID;constraint:OnDelete:CASCADE"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type SplitGroupInvite struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"index;not null" json:"user_id"`
	GroupID   uint       `gorm:"index;not null" json:"group_id"`
	Group     SplitGroup `json:"group,omitempty" gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE"`
	Token     string     `gorm:"uniqueIndex;not null" json:"token"`
	Status    string     `gorm:"type:varchar(16);not null;default:active" json:"status"`
	ExpiresAt *time.Time `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type SplitGroupDirectInvite struct {
	ID            uint             `gorm:"primaryKey" json:"id"`
	UserID        uint             `gorm:"index;not null" json:"user_id"`
	GroupID       uint             `gorm:"index;not null" json:"group_id"`
	Group         SplitGroup       `json:"group,omitempty" gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE"`
	InviteID      uint             `gorm:"index;not null" json:"invite_id"`
	Invite        SplitGroupInvite `json:"-" gorm:"foreignKey:InviteID;constraint:OnDelete:CASCADE"`
	TargetEmail   string           `gorm:"type:varchar(254);not null;default:''" json:"target_email"`
	TargetPhone   string           `gorm:"type:varchar(32);not null;default:''" json:"target_phone"`
	InvitedUserID *uint            `gorm:"index" json:"invited_user_id"`
	InvitedUser   *User            `json:"-" gorm:"foreignKey:InvitedUserID;constraint:OnDelete:SET NULL"`
	Status        string           `gorm:"type:varchar(16);not null;default:pending" json:"status"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

type SplitGroupUserMember struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	GroupID   uint       `gorm:"index;not null" json:"group_id"`
	Group     SplitGroup `json:"group,omitempty" gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE"`
	UserID    uint       `gorm:"index;not null" json:"user_id"`
	User      User       `json:"user,omitempty" gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Role      string     `gorm:"type:varchar(16);not null;default:member" json:"role"`
	Status    string     `gorm:"type:varchar(16);not null;default:active" json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type SplitBill struct {
	ID              uint               `gorm:"primaryKey" json:"id"`
	UserID          uint               `gorm:"index;not null" json:"user_id"`
	EntryID         *uint              `gorm:"index" json:"entry_id"`
	Entry           *Entry             `json:"entry,omitempty" gorm:"foreignKey:EntryID;constraint:OnDelete:SET NULL"`
	GroupID         *uint              `gorm:"index" json:"group_id"`
	Group           *SplitGroup        `json:"group,omitempty" gorm:"foreignKey:GroupID;constraint:OnDelete:SET NULL"`
	Title           string             `gorm:"not null" json:"title"`
	TotalAmount     Money              `gorm:"type:numeric(19,2);not null" json:"total_amount"`
	Currency        string             `gorm:"type:char(3);not null;default:INR" json:"currency"`
	Date            string             `gorm:"not null" json:"date"`
	Notes           string             `json:"notes"`
	Participants    []SplitParticipant `json:"participants,omitempty" gorm:"foreignKey:BillID"`
	ViewerCanEdit   bool               `gorm:"-" json:"viewer_can_edit,omitempty"`
	ViewerCanDelete bool               `gorm:"-" json:"viewer_can_delete,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
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
