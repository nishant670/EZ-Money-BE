package http

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"finance-parser-go/internal/database"
	"finance-parser-go/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	splitDirectionFriendOwesUser = "friend_owes_user"
	splitDirectionUserOwesFriend = "user_owes_friend"

	settlementDirectionFriendPaidUser = "friend_paid_user"
	settlementDirectionUserPaidFriend = "user_paid_friend"
)

type splitFriendInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type splitParticipantInput struct {
	FriendID    uint         `json:"friend_id"`
	ShareAmount models.Money `json:"share_amount"`
	Direction   string       `json:"direction"`
}

type splitGroupInput struct {
	Name      string `json:"name"`
	FriendIDs []uint `json:"friend_ids"`
}

type splitBillInput struct {
	EntryID      *uint                   `json:"entry_id"`
	GroupID      *uint                   `json:"group_id"`
	Title        string                  `json:"title"`
	TotalAmount  models.Money            `json:"total_amount"`
	Currency     string                  `json:"currency"`
	Date         string                  `json:"date"`
	Notes        string                  `json:"notes"`
	Participants []splitParticipantInput `json:"participants"`
}

type splitSettlementInput struct {
	FriendID  uint         `json:"friend_id"`
	Amount    models.Money `json:"amount"`
	Direction string       `json:"direction"`
	Date      string       `json:"date"`
	Notes     string       `json:"notes"`
}

type splitBalance struct {
	Friend            models.SplitFriend `json:"friend"`
	TotalOwedByFriend models.Money       `json:"total_owed_by_friend"`
	TotalOwedToFriend models.Money       `json:"total_owed_to_friend"`
	NetBalance        models.Money       `json:"net_balance"`
}

type splitActivityItem struct {
	ID               string                    `json:"id"`
	Type             string                    `json:"type"`
	RecordID         uint                      `json:"record_id"`
	Title            string                    `json:"title"`
	Date             string                    `json:"date"`
	Amount           *models.Money             `json:"amount,omitempty"`
	GroupID          *uint                     `json:"group_id,omitempty"`
	Group            *models.SplitGroup        `json:"group,omitempty"`
	FriendID         *uint                     `json:"friend_id,omitempty"`
	Friend           *models.SplitFriend       `json:"friend,omitempty"`
	Direction        string                    `json:"direction,omitempty"`
	ParticipantCount int                       `json:"participant_count,omitempty"`
	Participants     []models.SplitParticipant `json:"participants,omitempty"`
	Notes            string                    `json:"notes,omitempty"`
	CreatedAt        time.Time                 `json:"created_at"`
}

func (s *Server) createSplitFriend(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var input splitFriendInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if fields := input.validate(); len(fields) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_friend", "fields": fields})
		return
	}

	friend := input.toModel(userID)
	if err := database.DB.Create(&friend).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_create_split_friend"})
		return
	}
	c.JSON(http.StatusCreated, friend)
}

func (s *Server) listSplitFriends(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	query := database.DB.Where("user_id = ?", userID)
	if !strings.EqualFold(c.Query("status"), "all") {
		query = query.Where("archived = ?", false)
	}

	var friends []models.SplitFriend
	if err := query.Order("name asc, created_at desc").Find(&friends).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_list_split_friends"})
		return
	}
	c.JSON(http.StatusOK, friends)
}

func (s *Server) updateSplitFriend(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	var input splitFriendInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if fields := input.validate(); len(fields) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_friend", "fields": fields})
		return
	}

	var friend models.SplitFriend
	if err := ownedSplitFriends(database.DB, userID).Where("id = ?", id).First(&friend).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "split_friend_not_found"})
		return
	}
	input.apply(&friend)
	if err := database.DB.Save(&friend).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_update_split_friend"})
		return
	}
	c.JSON(http.StatusOK, friend)
}

func (s *Server) archiveSplitFriend(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	result := ownedSplitFriends(database.DB.Model(&models.SplitFriend{}), userID).
		Where("id = ?", id).
		Update("archived", true)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_archive_split_friend"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "split_friend_not_found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "split friend archived"})
}

func (s *Server) createSplitGroup(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var input splitGroupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if fields := input.validate(); len(fields) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_group", "fields": fields})
		return
	}
	if fields, err := validateSplitGroupFriends(userID, input.FriendIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "split_friend_lookup_failed"})
		return
	} else if len(fields) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_group", "fields": fields})
		return
	}

	var group models.SplitGroup
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		group = models.SplitGroup{UserID: userID, Name: strings.TrimSpace(input.Name)}
		if err := tx.Create(&group).Error; err != nil {
			return err
		}
		return createSplitGroupMembers(tx, userID, group.ID, input.FriendIDs)
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_create_split_group"})
		return
	}
	_ = database.DB.Preload("Members.Friend").First(&group, group.ID).Error
	c.JSON(http.StatusCreated, group)
}

func (s *Server) listSplitGroups(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	query := database.DB.Preload("Members.Friend").Where("user_id = ?", userID)
	if !strings.EqualFold(c.Query("status"), "all") {
		query = query.Where("archived = ?", false)
	}

	var groups []models.SplitGroup
	if err := query.Order("name asc, created_at desc").Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_list_split_groups"})
		return
	}
	c.JSON(http.StatusOK, groups)
}

func (s *Server) updateSplitGroup(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	var input splitGroupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if fields := input.validate(); len(fields) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_group", "fields": fields})
		return
	}
	if fields, err := validateSplitGroupFriends(userID, input.FriendIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "split_friend_lookup_failed"})
		return
	} else if len(fields) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_group", "fields": fields})
		return
	}

	var group models.SplitGroup
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := ownedSplitGroups(tx, userID).Where("id = ?", id).First(&group).Error; err != nil {
			return err
		}
		group.Name = strings.TrimSpace(input.Name)
		if err := tx.Save(&group).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND group_id = ?", userID, group.ID).Delete(&models.SplitGroupMember{}).Error; err != nil {
			return err
		}
		return createSplitGroupMembers(tx, userID, group.ID, input.FriendIDs)
	}); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "split_group_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_update_split_group"})
		return
	}
	_ = database.DB.Preload("Members.Friend").First(&group, group.ID).Error
	c.JSON(http.StatusOK, group)
}

func (s *Server) archiveSplitGroup(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	result := ownedSplitGroups(database.DB.Model(&models.SplitGroup{}), userID).
		Where("id = ?", id).
		Update("archived", true)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_archive_split_group"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "split_group_not_found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "split group archived"})
}

func (s *Server) createSplitBill(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var input splitBillInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if fields := input.validate(); len(fields) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_bill", "fields": fields})
		return
	}
	if input.EntryID != nil {
		if ok, err := userOwnsEntry(userID, *input.EntryID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "entry_lookup_failed"})
			return
		} else if !ok {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_bill", "fields": gin.H{"entry_id": "must belong to the current user"}})
			return
		}
	}
	if fields, err := validateSplitParticipantFriends(userID, input.Participants); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "split_friend_lookup_failed"})
		return
	} else if len(fields) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_bill", "fields": fields})
		return
	}
	if input.GroupID != nil {
		if ok, err := userOwnsActiveSplitGroup(userID, *input.GroupID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "split_group_lookup_failed"})
			return
		} else if !ok {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_bill", "fields": gin.H{"group_id": "must belong to the current user"}})
			return
		}
	}

	var bill models.SplitBill
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		bill = input.toModel(userID)
		if err := tx.Create(&bill).Error; err != nil {
			return err
		}
		participants := make([]models.SplitParticipant, 0, len(input.Participants))
		for _, participant := range input.Participants {
			participants = append(participants, participant.toModel(userID, bill.ID))
		}
		if err := tx.Create(&participants).Error; err != nil {
			return err
		}
		return tx.Preload("Group").Preload("Participants.Friend").First(&bill, bill.ID).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_create_split_bill"})
		return
	}

	c.JSON(http.StatusCreated, bill)
}

func (s *Server) listSplitBills(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var bills []models.SplitBill
	if err := database.DB.Preload("Group").Preload("Participants.Friend").
		Where("user_id = ?", userID).
		Order("date desc, created_at desc").
		Find(&bills).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_list_split_bills"})
		return
	}
	c.JSON(http.StatusOK, bills)
}

func (s *Server) updateSplitBill(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	var input splitBillInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if fields := input.validate(); len(fields) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_bill", "fields": fields})
		return
	}
	if input.EntryID != nil {
		if ok, err := userOwnsEntry(userID, *input.EntryID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "entry_lookup_failed"})
			return
		} else if !ok {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_bill", "fields": gin.H{"entry_id": "must belong to the current user"}})
			return
		}
	}
	if fields, err := validateSplitParticipantFriends(userID, input.Participants); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "split_friend_lookup_failed"})
		return
	} else if len(fields) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_bill", "fields": fields})
		return
	}
	if input.GroupID != nil {
		if ok, err := userOwnsActiveSplitGroup(userID, *input.GroupID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "split_group_lookup_failed"})
			return
		} else if !ok {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_bill", "fields": gin.H{"group_id": "must belong to the current user"}})
			return
		}
	}

	var bill models.SplitBill
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND id = ?", userID, id).First(&bill).Error; err != nil {
			return err
		}
		bill.EntryID = input.EntryID
		bill.GroupID = input.GroupID
		bill.Title = strings.TrimSpace(input.Title)
		bill.TotalAmount = input.TotalAmount
		bill.Currency = normalizedSplitCurrency(input.Currency)
		bill.Date = input.Date
		bill.Notes = strings.TrimSpace(input.Notes)
		if err := tx.Save(&bill).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND bill_id = ?", userID, bill.ID).Delete(&models.SplitParticipant{}).Error; err != nil {
			return err
		}
		participants := make([]models.SplitParticipant, 0, len(input.Participants))
		for _, participant := range input.Participants {
			participants = append(participants, participant.toModel(userID, bill.ID))
		}
		if err := tx.Create(&participants).Error; err != nil {
			return err
		}
		return tx.Preload("Group").Preload("Participants.Friend").First(&bill, bill.ID).Error
	}); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "split_bill_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_update_split_bill"})
		return
	}

	c.JSON(http.StatusOK, bill)
}

func (s *Server) deleteSplitBill(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND bill_id = ?", userID, id).Delete(&models.SplitParticipant{}).Error; err != nil {
			return err
		}
		result := tx.Where("user_id = ? AND id = ?", userID, id).Delete(&models.SplitBill{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "split_bill_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_delete_split_bill"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "split bill deleted"})
}

func (s *Server) createSplitSettlement(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var input splitSettlementInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	if fields := input.validate(); len(fields) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_settlement", "fields": fields})
		return
	}
	if ok, err := userOwnsActiveSplitFriend(userID, input.FriendID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "split_friend_lookup_failed"})
		return
	} else if !ok {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_settlement", "fields": gin.H{"friend_id": "must belong to the current user"}})
		return
	}

	settlement := input.toModel(userID)
	if err := database.DB.Create(&settlement).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_create_split_settlement"})
		return
	}
	_ = database.DB.Preload("Friend").First(&settlement, settlement.ID).Error
	c.JSON(http.StatusCreated, settlement)
}

func (s *Server) listSplitSettlements(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var settlements []models.SplitSettlement
	if err := database.DB.Preload("Friend").
		Where("user_id = ?", userID).
		Order("date desc, created_at desc").
		Find(&settlements).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_list_split_settlements"})
		return
	}
	c.JSON(http.StatusOK, settlements)
}

func (s *Server) listSplitActivity(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	page, pageSize := parseBillingPagination(c.Query("page"), c.Query("page_size"))

	var bills []models.SplitBill
	if err := database.DB.Preload("Group").Preload("Participants.Friend").
		Where("user_id = ?", userID).
		Find(&bills).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_list_split_activity"})
		return
	}
	var settlements []models.SplitSettlement
	if err := database.DB.Preload("Friend").
		Where("user_id = ?", userID).
		Find(&settlements).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_list_split_activity"})
		return
	}
	var groups []models.SplitGroup
	if err := database.DB.Preload("Members.Friend").
		Where("user_id = ?", userID).
		Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_list_split_activity"})
		return
	}
	var friends []models.SplitFriend
	if err := database.DB.
		Where("user_id = ?", userID).
		Find(&friends).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_list_split_activity"})
		return
	}

	items := make([]splitActivityItem, 0, len(bills)+len(settlements)+len(groups)+len(friends))
	for _, group := range groups {
		groupCopy := group
		items = append(items, splitActivityItem{
			ID:               fmt.Sprintf("group-%d", group.ID),
			Type:             "group_created",
			RecordID:         group.ID,
			Title:            fmt.Sprintf("%s created", group.Name),
			Date:             group.CreatedAt.Format("2006-01-02"),
			GroupID:          &group.ID,
			Group:            &groupCopy,
			ParticipantCount: len(group.Members),
			CreatedAt:        group.CreatedAt,
		})
	}
	for _, friend := range friends {
		friendID := friend.ID
		friendCopy := friend
		items = append(items, splitActivityItem{
			ID:        fmt.Sprintf("friend-%d", friend.ID),
			Type:      "friend_created",
			RecordID:  friend.ID,
			Title:     fmt.Sprintf("%s added", fallbackSplitFriendName(friend)),
			Date:      friend.CreatedAt.Format("2006-01-02"),
			FriendID:  &friendID,
			Friend:    &friendCopy,
			CreatedAt: friend.CreatedAt,
		})
	}
	for _, bill := range bills {
		amount := bill.TotalAmount
		item := splitActivityItem{
			ID:               fmt.Sprintf("bill-%d", bill.ID),
			Type:             "bill",
			RecordID:         bill.ID,
			Title:            bill.Title,
			Date:             bill.Date,
			Amount:           &amount,
			GroupID:          bill.GroupID,
			ParticipantCount: len(bill.Participants),
			Participants:     bill.Participants,
			Notes:            bill.Notes,
			CreatedAt:        bill.CreatedAt,
		}
		if bill.Group != nil && bill.Group.ID != 0 {
			item.Group = bill.Group
		}
		items = append(items, item)
	}
	for _, settlement := range settlements {
		friendID := settlement.FriendID
		amount := settlement.Amount
		title := "Settlement"
		if settlement.Direction == settlementDirectionFriendPaidUser {
			title = fmt.Sprintf("%s paid you", fallbackSplitFriendName(settlement.Friend))
		} else if settlement.Direction == settlementDirectionUserPaidFriend {
			title = fmt.Sprintf("You paid %s", fallbackSplitFriendName(settlement.Friend))
		}
		item := splitActivityItem{
			ID:        fmt.Sprintf("settlement-%d", settlement.ID),
			Type:      "settlement",
			RecordID:  settlement.ID,
			Title:     title,
			Date:      settlement.Date,
			Amount:    &amount,
			FriendID:  &friendID,
			Direction: settlement.Direction,
			Notes:     settlement.Notes,
			CreatedAt: settlement.CreatedAt,
		}
		if settlement.Friend.ID != 0 {
			friend := settlement.Friend
			item.Friend = &friend
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Date != items[j].Date {
			return items[i].Date > items[j].Date
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     items[start:end],
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

func (s *Server) listSplitBalances(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	balances, err := buildSplitBalances(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_list_split_balances"})
		return
	}
	c.JSON(http.StatusOK, balances)
}

func fallbackSplitFriendName(friend models.SplitFriend) string {
	if strings.TrimSpace(friend.Name) == "" {
		return "Friend"
	}
	return friend.Name
}

func (input splitFriendInput) validate() map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(input.Name) == "" {
		fields["name"] = "is required"
	}
	if len(strings.TrimSpace(input.Name)) > 120 {
		fields["name"] = "must not exceed 120 characters"
	}
	if len(strings.TrimSpace(input.Email)) > 254 {
		fields["email"] = "must not exceed 254 characters"
	}
	if len(strings.TrimSpace(input.Phone)) > 32 {
		fields["phone"] = "must not exceed 32 characters"
	}
	return fields
}

func (input splitFriendInput) toModel(userID uint) models.SplitFriend {
	return models.SplitFriend{
		UserID: userID,
		Name:   strings.TrimSpace(input.Name),
		Email:  strings.TrimSpace(input.Email),
		Phone:  strings.TrimSpace(input.Phone),
	}
}

func (input splitFriendInput) apply(friend *models.SplitFriend) {
	friend.Name = strings.TrimSpace(input.Name)
	friend.Email = strings.TrimSpace(input.Email)
	friend.Phone = strings.TrimSpace(input.Phone)
}

func (input splitGroupInput) validate() map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(input.Name) == "" {
		fields["name"] = "is required"
	}
	if len(strings.TrimSpace(input.Name)) > 120 {
		fields["name"] = "must not exceed 120 characters"
	}
	seen := map[uint]bool{}
	for index, friendID := range input.FriendIDs {
		if friendID == 0 {
			fields[fmt.Sprintf("friend_ids[%d]", index)] = "must be a positive integer"
		}
		if seen[friendID] {
			fields[fmt.Sprintf("friend_ids[%d]", index)] = "duplicate friend"
		}
		seen[friendID] = true
	}
	return fields
}

func (input splitBillInput) validate() map[string]string {
	fields := map[string]string{}
	if strings.TrimSpace(input.Title) == "" {
		fields["title"] = "is required"
	}
	if !input.TotalAmount.IsPositive() {
		fields["total_amount"] = "must be positive"
	}
	currency := normalizedSplitCurrency(input.Currency)
	if currency != "INR" {
		fields["currency"] = "must be INR"
	}
	if _, err := time.Parse("2006-01-02", input.Date); err != nil {
		fields["date"] = "must use YYYY-MM-DD"
	}
	if input.GroupID != nil && *input.GroupID == 0 {
		fields["group_id"] = "must be a positive integer"
	}
	if len(input.Participants) == 0 {
		fields["participants"] = "must include at least one friend share"
	}

	seen := map[string]bool{}
	totalShares := models.Money(0)
	for index, participant := range input.Participants {
		prefix := fmt.Sprintf("participants[%d]", index)
		if participant.FriendID == 0 {
			fields[prefix+".friend_id"] = "must be a positive integer"
		}
		if !participant.ShareAmount.IsPositive() {
			fields[prefix+".share_amount"] = "must be positive"
		}
		direction := normalizeSplitDirection(participant.Direction)
		if direction == "" {
			fields[prefix+".direction"] = "must be friend_owes_user or user_owes_friend"
		}
		key := fmt.Sprintf("%d:%s", participant.FriendID, direction)
		if participant.FriendID != 0 && direction != "" {
			if seen[key] {
				fields[prefix+".friend_id"] = "duplicate friend and direction"
			}
			seen[key] = true
		}
		totalShares += participant.ShareAmount
	}
	if totalShares > input.TotalAmount {
		fields["participants"] = "shares must not exceed total_amount"
	}
	return fields
}

func (input splitBillInput) toModel(userID uint) models.SplitBill {
	return models.SplitBill{
		UserID:      userID,
		EntryID:     input.EntryID,
		GroupID:     input.GroupID,
		Title:       strings.TrimSpace(input.Title),
		TotalAmount: input.TotalAmount,
		Currency:    normalizedSplitCurrency(input.Currency),
		Date:        input.Date,
		Notes:       strings.TrimSpace(input.Notes),
	}
}

func (input splitParticipantInput) toModel(userID, billID uint) models.SplitParticipant {
	return models.SplitParticipant{
		UserID:      userID,
		BillID:      billID,
		FriendID:    input.FriendID,
		ShareAmount: input.ShareAmount,
		Direction:   normalizeSplitDirection(input.Direction),
	}
}

func (input splitSettlementInput) validate() map[string]string {
	fields := map[string]string{}
	if input.FriendID == 0 {
		fields["friend_id"] = "must be a positive integer"
	}
	if !input.Amount.IsPositive() {
		fields["amount"] = "must be positive"
	}
	if normalizeSettlementDirection(input.Direction) == "" {
		fields["direction"] = "must be friend_paid_user or user_paid_friend"
	}
	if _, err := time.Parse("2006-01-02", input.Date); err != nil {
		fields["date"] = "must use YYYY-MM-DD"
	}
	return fields
}

func (input splitSettlementInput) toModel(userID uint) models.SplitSettlement {
	return models.SplitSettlement{
		UserID:    userID,
		FriendID:  input.FriendID,
		Amount:    input.Amount,
		Direction: normalizeSettlementDirection(input.Direction),
		Date:      input.Date,
		Notes:     strings.TrimSpace(input.Notes),
	}
}

func buildSplitBalances(db *gorm.DB, userID uint) ([]splitBalance, error) {
	var friends []models.SplitFriend
	if err := ownedSplitFriends(db, userID).Order("name asc, created_at desc").Find(&friends).Error; err != nil {
		return nil, err
	}

	balancesByFriend := map[uint]*splitBalance{}
	for _, friend := range friends {
		friend := friend
		balancesByFriend[friend.ID] = &splitBalance{Friend: friend}
	}

	var participants []models.SplitParticipant
	if err := db.Where("user_id = ?", userID).Find(&participants).Error; err != nil {
		return nil, err
	}
	for _, participant := range participants {
		balance := balancesByFriend[participant.FriendID]
		if balance == nil {
			continue
		}
		switch participant.Direction {
		case splitDirectionFriendOwesUser:
			balance.TotalOwedByFriend += participant.ShareAmount
			balance.NetBalance += participant.ShareAmount
		case splitDirectionUserOwesFriend:
			balance.TotalOwedToFriend += participant.ShareAmount
			balance.NetBalance -= participant.ShareAmount
		}
	}

	var settlements []models.SplitSettlement
	if err := db.Where("user_id = ?", userID).Find(&settlements).Error; err != nil {
		return nil, err
	}
	for _, settlement := range settlements {
		balance := balancesByFriend[settlement.FriendID]
		if balance == nil {
			continue
		}
		switch settlement.Direction {
		case settlementDirectionFriendPaidUser:
			balance.TotalOwedByFriend -= settlement.Amount
			balance.NetBalance -= settlement.Amount
		case settlementDirectionUserPaidFriend:
			balance.TotalOwedToFriend -= settlement.Amount
			balance.NetBalance += settlement.Amount
		}
	}

	result := make([]splitBalance, 0, len(friends))
	for _, friend := range friends {
		result = append(result, *balancesByFriend[friend.ID])
	}
	return result, nil
}

func validateSplitParticipantFriends(userID uint, participants []splitParticipantInput) (gin.H, error) {
	fields := gin.H{}
	for index, participant := range participants {
		if participant.FriendID == 0 {
			continue
		}
		ok, err := userOwnsActiveSplitFriend(userID, participant.FriendID)
		if err != nil {
			return nil, err
		}
		if !ok {
			fields[fmt.Sprintf("participants[%d].friend_id", index)] = "must belong to the current user"
		}
	}
	return fields, nil
}

func validateSplitGroupFriends(userID uint, friendIDs []uint) (gin.H, error) {
	fields := gin.H{}
	for index, friendID := range friendIDs {
		if friendID == 0 {
			continue
		}
		ok, err := userOwnsActiveSplitFriend(userID, friendID)
		if err != nil {
			return nil, err
		}
		if !ok {
			fields[fmt.Sprintf("friend_ids[%d]", index)] = "must belong to the current user"
		}
	}
	return fields, nil
}

func validateEntrySplitReferences(userID uint, input *entrySplitInput) (gin.H, error) {
	fields := gin.H{}
	if input == nil {
		return fields, nil
	}
	if input.GroupID != nil {
		ok, err := userOwnsActiveSplitGroup(userID, *input.GroupID)
		if err != nil {
			return nil, err
		}
		if !ok {
			fields["split.group_id"] = "must belong to the current user"
		}
	}
	for index, participant := range input.Participants {
		if participant.FriendID == nil {
			continue
		}
		ok, err := userOwnsActiveSplitFriend(userID, *participant.FriendID)
		if err != nil {
			return nil, err
		}
		if !ok {
			fields[fmt.Sprintf("split.participants[%d].friend_id", index)] = "must belong to the current user"
		}
	}
	return fields, nil
}

func createEntrySplitBill(tx *gorm.DB, userID uint, entry models.Entry, input *entrySplitInput) error {
	if input == nil {
		return nil
	}

	friendIDs := make([]uint, 0, len(input.Participants))
	participants := make([]models.SplitParticipant, 0, len(input.Participants))
	for _, participant := range input.Participants {
		friendID := uint(0)
		if participant.FriendID != nil {
			friendID = *participant.FriendID
		} else {
			friend := participant.Friend.toModel(userID)
			if err := tx.Create(&friend).Error; err != nil {
				return err
			}
			friendID = friend.ID
		}
		friendIDs = append(friendIDs, friendID)
		direction := normalizeSplitDirection(participant.Direction)
		if direction == "" {
			direction = splitDirectionFriendOwesUser
		}
		participants = append(participants, models.SplitParticipant{
			UserID:      userID,
			FriendID:    friendID,
			ShareAmount: participant.ShareAmount,
			Direction:   direction,
		})
	}

	groupID := input.GroupID
	if groupID == nil && strings.TrimSpace(input.GroupName) != "" {
		group := models.SplitGroup{UserID: userID, Name: strings.TrimSpace(input.GroupName)}
		if err := tx.Create(&group).Error; err != nil {
			return err
		}
		groupID = &group.ID
	}
	if groupID != nil {
		if err := createSplitGroupMembers(tx, userID, *groupID, friendIDs); err != nil {
			return err
		}
	}

	entryID := entry.ID
	bill := models.SplitBill{
		UserID:      userID,
		EntryID:     &entryID,
		GroupID:     groupID,
		Title:       entry.Title,
		TotalAmount: entry.Amount,
		Currency:    entry.Currency,
		Date:        entry.Date,
		Notes:       strings.TrimSpace(input.Notes),
	}
	if bill.Title == "" {
		bill.Title = "Split transaction"
	}
	if bill.Currency == "" {
		bill.Currency = "INR"
	}
	if err := tx.Create(&bill).Error; err != nil {
		return err
	}
	for index := range participants {
		participants[index].BillID = bill.ID
	}
	return tx.Create(&participants).Error
}

func createSplitGroupMembers(tx *gorm.DB, userID, groupID uint, friendIDs []uint) error {
	seen := map[uint]bool{}
	members := make([]models.SplitGroupMember, 0, len(friendIDs))
	for _, friendID := range friendIDs {
		if friendID == 0 || seen[friendID] {
			continue
		}
		seen[friendID] = true
		var count int64
		if err := tx.Model(&models.SplitGroupMember{}).
			Where("user_id = ? AND group_id = ? AND friend_id = ?", userID, groupID, friendID).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		members = append(members, models.SplitGroupMember{
			UserID:   userID,
			GroupID:  groupID,
			FriendID: friendID,
		})
	}
	if len(members) == 0 {
		return nil
	}
	return tx.Create(&members).Error
}

func normalizedSplitCurrency(currency string) string {
	if strings.TrimSpace(currency) == "" {
		return "INR"
	}
	return strings.ToUpper(strings.TrimSpace(currency))
}

func normalizeSplitDirection(direction string) string {
	switch strings.ToLower(strings.TrimSpace(direction)) {
	case splitDirectionFriendOwesUser:
		return splitDirectionFriendOwesUser
	case splitDirectionUserOwesFriend:
		return splitDirectionUserOwesFriend
	default:
		return ""
	}
}

func normalizeSettlementDirection(direction string) string {
	switch strings.ToLower(strings.TrimSpace(direction)) {
	case settlementDirectionFriendPaidUser:
		return settlementDirectionFriendPaidUser
	case settlementDirectionUserPaidFriend:
		return settlementDirectionUserPaidFriend
	default:
		return ""
	}
}

func userOwnsActiveSplitFriend(userID, friendID uint) (bool, error) {
	var count int64
	err := ownedSplitFriends(database.DB.Model(&models.SplitFriend{}), userID).
		Where("id = ? AND archived = ?", friendID, false).
		Count(&count).Error
	return count == 1, err
}

func userOwnsActiveSplitGroup(userID, groupID uint) (bool, error) {
	var count int64
	err := ownedSplitGroups(database.DB.Model(&models.SplitGroup{}), userID).
		Where("id = ? AND archived = ?", groupID, false).
		Count(&count).Error
	return count == 1, err
}

func userOwnsEntry(userID, entryID uint) (bool, error) {
	var count int64
	err := ownedEntries(database.DB.Model(&models.Entry{}), userID).
		Where("id = ?", entryID).
		Count(&count).Error
	return count == 1, err
}

func ownedSplitFriends(db *gorm.DB, userID uint) *gorm.DB {
	return db.Where("user_id = ?", userID)
}

func ownedSplitGroups(db *gorm.DB, userID uint) *gorm.DB {
	return db.Where("user_id = ?", userID)
}

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 32)
	if err != nil || value == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return uint(value), true
}
