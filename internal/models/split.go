package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type SplitFriend struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	UserID   uint   `gorm:"index;not null" json:"user_id"`
	Name     string `gorm:"not null" json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Archived bool   `gorm:"not null;default:false" json:"archived"`
	// Set when the person behind this friend row has their own Finnri account.
	// A friend row belongs to whoever wrote it, so this is the only link that
	// lets a shared group say "this member is you" to the right viewer.
	LinkedUserID *uint     `gorm:"index" json:"linked_user_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SplitGroupDefaultSplitSlot names the group owner in a default split. Every
// other slot is a group member's friend id rendered as a string.
const SplitGroupDefaultSplitOwnerSlot = "owner"

// SplitGroupDefaultSplit is the group-wide split every new expense starts from.
//
// It is deliberately anchored on the owner rather than on "you": the row is
// read by every member, and a viewer-relative default would mean a different
// thing to each of them. Slots are the group owner plus the member friend ids,
// which is the same namespace group bills already validate participants
// against, so the stored default names the same people for everybody.
type SplitGroupDefaultSplit struct {
	// Payer is a slot: the owner, or a member friend id.
	Payer string `json:"payer"`
	// FullAmount is the "is owed the full amount" split, where the payer
	// carries none of the expense.
	FullAmount bool `json:"full_amount"`
	// Tab is one of equally, percentages, shares.
	Tab          string                        `json:"tab"`
	Participants []SplitGroupDefaultSplitShare `json:"participants"`
}

type SplitGroupDefaultSplitShare struct {
	Slot   string `json:"slot"`
	Weight string `json:"weight"`
}

// The default split is stored as one JSON document rather than a serializer
// tag: it is written through single-column updates as well as full saves, and
// only a Valuer/Scanner pair covers both paths.
func (split *SplitGroupDefaultSplit) Value() (driver.Value, error) {
	if split == nil {
		return nil, nil
	}
	raw, err := json.Marshal(*split)
	if err != nil {
		return nil, err
	}
	return string(raw), nil
}

func (split *SplitGroupDefaultSplit) Scan(value any) error {
	if split == nil {
		return errors.New("split: cannot scan a default split into a nil pointer")
	}
	switch typed := value.(type) {
	case nil:
		*split = SplitGroupDefaultSplit{}
		return nil
	case []byte:
		return json.Unmarshal(typed, split)
	case string:
		return json.Unmarshal([]byte(typed), split)
	default:
		return fmt.Errorf("split: cannot scan %T into a default split", value)
	}
}

type SplitGroup struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	UserID   uint   `gorm:"index;not null" json:"user_id"`
	Name     string `gorm:"not null" json:"name"`
	Archived bool   `gorm:"not null;default:false" json:"archived"`
	// Kind is what the group is for — trip, home, couple, other. It shapes the
	// group screen (only a trip has dates), so it belongs to the group rather
	// than to whoever happens to be looking at it.
	Kind string `gorm:"type:varchar(16);not null;default:other" json:"kind"`
	// DefaultSplit is stored as JSON text rather than a column per field: it is
	// read and written whole, and the shape is the client's to interpret.
	DefaultSplit *SplitGroupDefaultSplit `gorm:"type:text" json:"default_split"`
	Members      []SplitGroupMember      `json:"members,omitempty" gorm:"foreignKey:GroupID"`
	// What happened to the people just added, so the app never implies somebody
	// was told when nobody could be. Response-only, and only on a save.
	MemberInvites       []SplitGroupMemberInvite `gorm:"-" json:"member_invites,omitempty"`
	OwnerName           string                   `gorm:"-" json:"owner_name,omitempty"`
	ViewerFriendID      *uint                    `gorm:"-" json:"viewer_friend_id,omitempty"`
	ViewerRole          string                   `gorm:"-" json:"viewer_role,omitempty"`
	ViewerCanAddExpense bool                     `gorm:"-" json:"viewer_can_add_expense,omitempty"`
	ViewerCanManage     bool                     `gorm:"-" json:"viewer_can_manage,omitempty"`
	CreatedAt           time.Time                `json:"created_at"`
	UpdatedAt           time.Time                `json:"updated_at"`
}

// SplitGroupMemberInviteStatus values.
const (
	// The person has a Finnri account and now has an invite waiting in it.
	SplitMemberInviteNotified = "notified"
	// No account yet: the invite exists but the link has to be handed over.
	SplitMemberInviteLinkNeeded = "invite_created"
	// The friend row carries no email or phone, so there was nobody to reach.
	SplitMemberInviteNoContact = "no_contact"
)

type SplitGroupMemberInvite struct {
	FriendID uint   `json:"friend_id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
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
	ID       uint             `gorm:"primaryKey" json:"id"`
	UserID   uint             `gorm:"index;not null" json:"user_id"`
	GroupID  uint             `gorm:"index;not null" json:"group_id"`
	Group    SplitGroup       `json:"group,omitempty" gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE"`
	InviteID uint             `gorm:"index;not null" json:"invite_id"`
	Invite   SplitGroupInvite `json:"-" gorm:"foreignKey:InviteID;constraint:OnDelete:CASCADE"`
	// Which friend row this invite was raised for, when it came from adding a
	// specific person to the group. It is what lets acceptance reuse the row the
	// owner has already been splitting against instead of guessing at one.
	FriendID      *uint     `gorm:"index" json:"friend_id,omitempty"`
	TargetEmail   string    `gorm:"type:varchar(254);not null;default:''" json:"target_email"`
	TargetPhone   string    `gorm:"type:varchar(32);not null;default:''" json:"target_phone"`
	InvitedUserID *uint     `gorm:"index" json:"invited_user_id"`
	InvitedUser   *User     `json:"-" gorm:"foreignKey:InvitedUserID;constraint:OnDelete:SET NULL"`
	Status        string    `gorm:"type:varchar(16);not null;default:pending" json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
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
