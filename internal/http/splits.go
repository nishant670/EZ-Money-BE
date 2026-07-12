package http

import (
	"fmt"
	"net/http"
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

type splitBillInput struct {
	EntryID      *uint                   `json:"entry_id"`
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
		return tx.Preload("Participants.Friend").First(&bill, bill.ID).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_create_split_bill"})
		return
	}

	c.JSON(http.StatusCreated, bill)
}

func (s *Server) listSplitBills(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	var bills []models.SplitBill
	if err := database.DB.Preload("Participants.Friend").
		Where("user_id = ?", userID).
		Order("date desc, created_at desc").
		Find(&bills).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_list_split_bills"})
		return
	}
	c.JSON(http.StatusOK, bills)
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

func (s *Server) listSplitBalances(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	balances, err := buildSplitBalances(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_list_split_balances"})
		return
	}
	c.JSON(http.StatusOK, balances)
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

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 32)
	if err != nil || value == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, false
	}
	return uint(value), true
}
