package http

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
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

type splitGroupDirectInviteInput struct {
	Email string `json:"email"`
	Phone string `json:"phone"`
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

type splitGroupInviteResponse struct {
	Token     string            `json:"token"`
	URL       string            `json:"url"`
	DeepLink  string            `json:"deep_link"`
	Group     models.SplitGroup `json:"group"`
	ExpiresAt *time.Time        `json:"expires_at"`
}

type splitGroupDirectInviteResponse struct {
	ID               uint              `json:"id"`
	TargetEmail      string            `json:"target_email"`
	TargetPhone      string            `json:"target_phone"`
	MatchedUser      bool              `json:"matched_user"`
	NotificationSent bool              `json:"notification_sent"`
	URL              string            `json:"url"`
	DeepLink         string            `json:"deep_link"`
	Message          string            `json:"message"`
	Status           string            `json:"status"`
	Group            models.SplitGroup `json:"group"`
	CreatedAt        time.Time         `json:"created_at"`
}

type splitGroupInviteDetailsResponse struct {
	Token       string            `json:"token"`
	Group       models.SplitGroup `json:"group"`
	OwnerName   string            `json:"owner_name"`
	MemberCount int               `json:"member_count"`
	Status      string            `json:"status"`
	ExpiresAt   *time.Time        `json:"expires_at"`
}

type splitGroupInviteAcceptResponse struct {
	Group  models.SplitGroup       `json:"group"`
	Friend models.SplitFriend      `json:"friend"`
	Member models.SplitGroupMember `json:"member"`
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

func (s *Server) getSplitGroupInvite(c *gin.Context) {
	invite, group, owner, ok := loadActiveSplitGroupInvite(c)
	if !ok {
		return
	}

	c.JSON(http.StatusOK, splitGroupInviteDetailsResponse{
		Token:       invite.Token,
		Group:       group,
		OwnerName:   displayNameForUser(owner),
		MemberCount: len(group.Members) + 1,
		Status:      invite.Status,
		ExpiresAt:   invite.ExpiresAt,
	})
}

func (s *Server) acceptSplitGroupInvite(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	_, group, owner, ok := loadActiveSplitGroupInvite(c)
	if !ok {
		return
	}
	if owner.ID == userID {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "cannot_accept_own_split_group_invite"})
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_not_found"})
		return
	}

	var friend models.SplitFriend
	var member models.SplitGroupMember
	var userMember models.SplitGroupUserMember
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		query := tx.Where("user_id = ? AND archived = ?", owner.ID, false)
		identityQuery, identityArgs := splitInviteUserIdentityQuery(user)
		if identityQuery != "" {
			query = query.Where(identityQuery, identityArgs...)
		} else {
			query = query.Where("name = ?", displayNameForUser(user))
		}
		err := query.First(&friend).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err == gorm.ErrRecordNotFound {
			friend = models.SplitFriend{
				UserID: owner.ID,
				Name:   displayNameForUser(user),
				Email:  stringFromPointer(user.Email),
				Phone:  stringFromPointer(user.Phone),
			}
			if err := tx.Create(&friend).Error; err != nil {
				return err
			}
		}

		err = tx.Where("user_id = ? AND group_id = ? AND friend_id = ?", owner.ID, group.ID, friend.ID).
			First(&member).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err == gorm.ErrRecordNotFound {
			member = models.SplitGroupMember{
				UserID:   owner.ID,
				GroupID:  group.ID,
				FriendID: friend.ID,
			}
			if err := tx.Create(&member).Error; err != nil {
				return err
			}
		}
		err = tx.Where("group_id = ? AND user_id = ?", group.ID, user.ID).
			First(&userMember).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err == gorm.ErrRecordNotFound {
			userMember = models.SplitGroupUserMember{
				GroupID: group.ID,
				UserID:  user.ID,
				Role:    "member",
				Status:  "active",
			}
			if err := tx.Create(&userMember).Error; err != nil {
				return err
			}
		} else if userMember.Status != "active" || userMember.Role != "member" {
			userMember.Status = "active"
			userMember.Role = "member"
			if err := tx.Save(&userMember).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_accept_split_group_invite"})
		return
	}

	_ = createNotification(
		owner.ID,
		"split.group_invite.accepted",
		fmt.Sprintf("%s joined %s", displayNameForUser(user), group.Name),
		fmt.Sprintf("%s accepted your Finnri split group invite.", displayNameForUser(user)),
		fmt.Sprintf("/split/groups/%d", group.ID),
	)

	_ = database.DB.Preload("Members.Friend").First(&group, group.ID).Error
	applySplitGroupViewerPermissions(&group, userID)
	c.JSON(http.StatusOK, splitGroupInviteAcceptResponse{
		Group:  group,
		Friend: friend,
		Member: member,
	})
}

func loadActiveSplitGroupInvite(c *gin.Context) (models.SplitGroupInvite, models.SplitGroup, models.User, bool) {
	token := strings.TrimSpace(c.Param("token"))
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_invite_token"})
		return models.SplitGroupInvite{}, models.SplitGroup{}, models.User{}, false
	}

	var invite models.SplitGroupInvite
	if err := database.DB.Where("token = ? AND status = ?", token, "active").First(&invite).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "split_group_invite_not_found"})
		return models.SplitGroupInvite{}, models.SplitGroup{}, models.User{}, false
	}
	if invite.ExpiresAt != nil && invite.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusGone, gin.H{"error": "split_group_invite_expired"})
		return models.SplitGroupInvite{}, models.SplitGroup{}, models.User{}, false
	}

	var group models.SplitGroup
	if err := database.DB.Preload("Members.Friend").
		Where("id = ? AND archived = ?", invite.GroupID, false).
		First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "split_group_not_found"})
		return models.SplitGroupInvite{}, models.SplitGroup{}, models.User{}, false
	}

	var owner models.User
	if err := database.DB.First(&owner, invite.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "split_group_owner_not_found"})
		return models.SplitGroupInvite{}, models.SplitGroup{}, models.User{}, false
	}

	return invite, group, owner, true
}

func (s *Server) createSplitGroupInvite(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	groupID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	var group models.SplitGroup
	if err := ownedSplitGroups(database.DB.Preload("Members.Friend"), userID).
		Where("id = ?", groupID).
		First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "split_group_not_found"})
		return
	}

	invite, err := getOrCreateActiveSplitGroupInvite(database.DB, userID, group.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_create_split_group_invite"})
		return
	}

	applySplitGroupViewerPermissions(&group, userID)
	c.JSON(http.StatusOK, splitGroupInviteResponse{
		Token:     invite.Token,
		URL:       splitInviteURL(invite.Token),
		DeepLink:  splitInviteDeepLink(invite.Token),
		Group:     group,
		ExpiresAt: invite.ExpiresAt,
	})
}

func (s *Server) createSplitGroupDirectInvite(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	groupID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	var input splitGroupDirectInviteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_json"})
		return
	}
	targetEmail := strings.TrimSpace(input.Email)
	targetPhone := strings.TrimSpace(input.Phone)
	if targetEmail == "" && targetPhone == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_group_invite", "fields": gin.H{"email": "email or phone is required"}})
		return
	}
	if targetEmail != "" {
		identifierType, normalized, err := normalizeIdentifier(targetEmail)
		if err != nil || identifierType != "email" || len(normalized) > 254 {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_group_invite", "fields": gin.H{"email": "must be a valid email"}})
			return
		}
		targetEmail = normalized
	}
	if targetPhone != "" {
		identifierType, normalized, err := normalizeIdentifier(targetPhone)
		if err != nil || identifierType != "phone" || len(normalized) > 32 {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_group_invite", "fields": gin.H{"phone": "must be a valid phone"}})
			return
		}
		targetPhone = normalized
	}

	var group models.SplitGroup
	if err := ownedSplitGroups(database.DB.Preload("Members.Friend"), userID).
		Where("id = ?", groupID).
		First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "split_group_not_found"})
		return
	}

	var owner models.User
	if err := database.DB.First(&owner, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_not_found"})
		return
	}

	var invitedUser models.User
	var invitedUserID *uint
	matchedUser := false
	userQuery := database.DB
	if targetEmail != "" && targetPhone != "" {
		userQuery = userQuery.Where("LOWER(email) = ? OR phone = ?", targetEmail, targetPhone)
	} else if targetEmail != "" {
		userQuery = userQuery.Where("LOWER(email) = ?", targetEmail)
	} else {
		userQuery = userQuery.Where("phone = ?", targetPhone)
	}
	if err := userQuery.First(&invitedUser).Error; err != nil && err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_lookup_invited_user"})
		return
	} else if err == nil {
		if invitedUser.ID == userID {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "cannot_invite_self_to_split_group"})
			return
		}
		matchedUser = true
		invitedUserID = &invitedUser.ID
	}

	invite, err := getOrCreateActiveSplitGroupInvite(database.DB, userID, group.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_create_split_group_invite"})
		return
	}

	directInvite, err := getOrCreateSplitGroupDirectInvite(database.DB, userID, group.ID, invite.ID, targetEmail, targetPhone, invitedUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_create_split_group_direct_invite"})
		return
	}

	url := splitInviteURL(invite.Token)
	deepLink := splitInviteDeepLink(invite.Token)
	message := fmt.Sprintf("%s invited you to join %s on Finnri to track shared expenses: %s", displayNameForUser(owner), group.Name, url)
	notificationSent := false
	if matchedUser {
		if err := createNotification(
			invitedUser.ID,
			"split.group_invite.received",
			fmt.Sprintf("Join %s on Finnri", group.Name),
			fmt.Sprintf("%s invited you to a split group.", displayNameForUser(owner)),
			fmt.Sprintf("/invite/split/%s", invite.Token),
		); err == nil {
			notificationSent = true
		}
	}

	applySplitGroupViewerPermissions(&group, userID)
	response := splitGroupDirectInviteToResponse(directInvite, group, owner)
	response.MatchedUser = matchedUser
	response.NotificationSent = notificationSent
	response.URL = url
	response.DeepLink = deepLink
	response.Message = message
	c.JSON(http.StatusCreated, response)
}

func (s *Server) listSplitGroupDirectInvites(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	groupID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	var group models.SplitGroup
	if err := ownedSplitGroups(database.DB, userID).
		Where("id = ? AND archived = ?", groupID, false).
		First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "split_group_not_found"})
		return
	}
	var owner models.User
	if err := database.DB.First(&owner, userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_not_found"})
		return
	}

	var invites []models.SplitGroupDirectInvite
	if err := database.DB.
		Preload("Invite").
		Where("user_id = ? AND group_id = ? AND status = ?", userID, group.ID, "pending").
		Order("created_at desc").
		Find(&invites).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_list_split_group_invites"})
		return
	}

	responses := make([]splitGroupDirectInviteResponse, 0, len(invites))
	for _, invite := range invites {
		responses = append(responses, splitGroupDirectInviteToResponse(invite, group, owner))
	}
	c.JSON(http.StatusOK, responses)
}

func (s *Server) revokeSplitGroupDirectInvite(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	groupID, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	inviteID, ok := parseUintParam(c, "invite_id")
	if !ok {
		return
	}

	var group models.SplitGroup
	if err := ownedSplitGroups(database.DB, userID).
		Where("id = ? AND archived = ?", groupID, false).
		First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "split_group_not_found"})
		return
	}

	result := database.DB.Model(&models.SplitGroupDirectInvite{}).
		Where("id = ? AND user_id = ? AND group_id = ? AND status = ?", inviteID, userID, group.ID, "pending").
		Update("status", "revoked")
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_revoke_split_group_invite"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "split_group_invite_not_found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "split group invite revoked"})
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
	applySplitGroupViewerPermissions(&group, userID)
	c.JSON(http.StatusCreated, group)
}

func (s *Server) listSplitGroups(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	sharedGroupIDs, err := activeSharedSplitGroupIDs(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_list_split_groups"})
		return
	}

	query := database.DB.Preload("Members.Friend")
	if len(sharedGroupIDs) > 0 {
		query = query.Where("user_id = ? OR id IN ?", userID, sharedGroupIDs)
	} else {
		query = query.Where("user_id = ?", userID)
	}
	if !strings.EqualFold(c.Query("status"), "all") {
		query = query.Where("archived = ?", false)
	}

	var groups []models.SplitGroup
	if err := query.Order("name asc, created_at desc").Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_list_split_groups"})
		return
	}
	applySplitGroupListViewerPermissions(groups, userID)
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
	applySplitGroupViewerPermissions(&group, userID)
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

func (s *Server) leaveSplitGroup(c *gin.Context) {
	userID := c.MustGet("userID").(uint)
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}

	var group models.SplitGroup
	if err := database.DB.Where("id = ? AND archived = ?", id, false).First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "split_group_not_found"})
		return
	}
	if group.UserID == userID {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "split_group_owner_cannot_leave"})
		return
	}

	result := database.DB.Model(&models.SplitGroupUserMember{}).
		Where("group_id = ? AND user_id = ? AND status = ?", id, userID, "active").
		Update("status", "removed")
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_leave_split_group"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "split_group_membership_not_found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "split group left"})
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
	if fields, err := validateSplitBillParticipantFriends(userID, input.GroupID, input.Participants); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "split_friend_lookup_failed"})
		return
	} else if len(fields) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_bill", "fields": fields})
		return
	}
	if input.GroupID != nil {
		if ok, err := userCanAccessActiveSplitGroup(userID, *input.GroupID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "split_group_lookup_failed"})
			return
		} else if !ok {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_bill", "fields": gin.H{"group_id": "must be a group you can access"}})
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

	applySplitBillViewerPermissions(&bill, userID)
	c.JSON(http.StatusCreated, bill)
}

func (s *Server) listSplitBills(c *gin.Context) {
	userID := c.MustGet("userID").(uint)

	accessibleGroupIDs, err := accessibleActiveSplitGroupIDs(database.DB, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_list_split_bills"})
		return
	}

	query := database.DB.Preload("Group").Preload("Participants.Friend")
	if len(accessibleGroupIDs) > 0 {
		query = query.Where("(user_id = ? AND group_id IS NULL) OR group_id IN ?", userID, accessibleGroupIDs)
	} else {
		query = query.Where("user_id = ? AND group_id IS NULL", userID)
	}

	var bills []models.SplitBill
	if err := query.
		Order("date desc, created_at desc").
		Find(&bills).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed_list_split_bills"})
		return
	}
	applySplitBillListViewerPermissions(bills, userID)
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
	if fields, err := validateSplitBillParticipantFriends(userID, input.GroupID, input.Participants); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "split_friend_lookup_failed"})
		return
	} else if len(fields) > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_bill", "fields": fields})
		return
	}
	if input.GroupID != nil {
		if ok, err := userCanAccessActiveSplitGroup(userID, *input.GroupID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "split_group_lookup_failed"})
			return
		} else if !ok {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_split_bill", "fields": gin.H{"group_id": "must be a group you can access"}})
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

	applySplitBillViewerPermissions(&bill, userID)
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

func validateSplitBillParticipantFriends(userID uint, groupID *uint, participants []splitParticipantInput) (gin.H, error) {
	if groupID == nil {
		return validateSplitParticipantFriends(userID, participants)
	}

	var groupFriendIDs []uint
	if err := database.DB.Model(&models.SplitGroupMember{}).
		Where("group_id = ?", *groupID).
		Pluck("friend_id", &groupFriendIDs).Error; err != nil {
		return nil, err
	}
	allowedFriendIDs := map[uint]bool{}
	for _, friendID := range groupFriendIDs {
		allowedFriendIDs[friendID] = true
	}

	fields := gin.H{}
	for index, participant := range participants {
		if participant.FriendID == 0 {
			continue
		}
		if !allowedFriendIDs[participant.FriendID] {
			fields[fmt.Sprintf("participants[%d].friend_id", index)] = "must belong to this group"
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

func applySplitGroupViewerPermissions(group *models.SplitGroup, viewerUserID uint) {
	if group == nil {
		return
	}
	group.ViewerCanAddExpense = true
	if group.UserID == viewerUserID {
		group.ViewerRole = "owner"
		group.ViewerCanManage = true
		return
	}
	group.ViewerRole = "member"
	group.ViewerCanManage = false
}

func applySplitGroupListViewerPermissions(groups []models.SplitGroup, viewerUserID uint) {
	for index := range groups {
		applySplitGroupViewerPermissions(&groups[index], viewerUserID)
	}
}

func applySplitBillViewerPermissions(bill *models.SplitBill, viewerUserID uint) {
	if bill == nil {
		return
	}
	canModify := bill.UserID == viewerUserID
	bill.ViewerCanEdit = canModify
	bill.ViewerCanDelete = canModify
	if bill.Group != nil {
		applySplitGroupViewerPermissions(bill.Group, viewerUserID)
	}
}

func applySplitBillListViewerPermissions(bills []models.SplitBill, viewerUserID uint) {
	for index := range bills {
		applySplitBillViewerPermissions(&bills[index], viewerUserID)
	}
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

func replaceEntrySplitBill(tx *gorm.DB, userID uint, entry models.Entry, input *entrySplitInput) error {
	if err := deleteEntrySplitBills(tx, userID, entry.ID); err != nil {
		return err
	}
	return createEntrySplitBill(tx, userID, entry, input)
}

func deleteEntrySplitBills(tx *gorm.DB, userID, entryID uint) error {
	var billIDs []uint
	if err := tx.Model(&models.SplitBill{}).
		Where("user_id = ? AND entry_id = ?", userID, entryID).
		Pluck("id", &billIDs).Error; err != nil {
		return err
	}
	if len(billIDs) == 0 {
		return nil
	}
	if err := tx.Where("user_id = ? AND bill_id IN ?", userID, billIDs).
		Delete(&models.SplitParticipant{}).Error; err != nil {
		return err
	}
	return tx.Where("user_id = ? AND id IN ?", userID, billIDs).
		Delete(&models.SplitBill{}).Error
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

func generateSplitInviteToken() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func getOrCreateActiveSplitGroupInvite(db *gorm.DB, userID, groupID uint) (models.SplitGroupInvite, error) {
	var invite models.SplitGroupInvite
	err := db.
		Where("user_id = ? AND group_id = ? AND status = ?", userID, groupID, "active").
		Order("created_at desc").
		First(&invite).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return models.SplitGroupInvite{}, err
	}
	if err == nil {
		return invite, nil
	}

	token, tokenErr := generateSplitInviteToken()
	if tokenErr != nil {
		return models.SplitGroupInvite{}, tokenErr
	}
	invite = models.SplitGroupInvite{
		UserID:  userID,
		GroupID: groupID,
		Token:   token,
		Status:  "active",
	}
	if err := db.Create(&invite).Error; err != nil {
		return models.SplitGroupInvite{}, err
	}
	return invite, nil
}

func getOrCreateSplitGroupDirectInvite(db *gorm.DB, userID, groupID, inviteID uint, targetEmail, targetPhone string, invitedUserID *uint) (models.SplitGroupDirectInvite, error) {
	query := db.Where("user_id = ? AND group_id = ? AND status = ?", userID, groupID, "pending")
	if targetEmail != "" {
		query = query.Where("LOWER(target_email) = ?", strings.ToLower(targetEmail))
	} else {
		query = query.Where("target_phone = ?", targetPhone)
	}

	var directInvite models.SplitGroupDirectInvite
	err := query.First(&directInvite).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return models.SplitGroupDirectInvite{}, err
	}
	if err == nil {
		changed := false
		if directInvite.InviteID != inviteID {
			directInvite.InviteID = inviteID
			changed = true
		}
		if invitedUserID != nil && (directInvite.InvitedUserID == nil || *directInvite.InvitedUserID != *invitedUserID) {
			directInvite.InvitedUserID = invitedUserID
			changed = true
		}
		if changed {
			return directInvite, db.Save(&directInvite).Error
		}
		return directInvite, nil
	}

	directInvite = models.SplitGroupDirectInvite{
		UserID:        userID,
		GroupID:       groupID,
		InviteID:      inviteID,
		TargetEmail:   targetEmail,
		TargetPhone:   targetPhone,
		InvitedUserID: invitedUserID,
		Status:        "pending",
	}
	if err := db.Create(&directInvite).Error; err != nil {
		return models.SplitGroupDirectInvite{}, err
	}
	return directInvite, nil
}

func splitGroupDirectInviteToResponse(invite models.SplitGroupDirectInvite, group models.SplitGroup, owner models.User) splitGroupDirectInviteResponse {
	token := invite.Invite.Token
	url := ""
	deepLink := ""
	message := ""
	if token != "" {
		url = splitInviteURL(token)
		deepLink = splitInviteDeepLink(token)
		message = fmt.Sprintf("%s invited you to join %s on Finnri to track shared expenses: %s", displayNameForUser(owner), group.Name, url)
	}
	return splitGroupDirectInviteResponse{
		ID:          invite.ID,
		TargetEmail: invite.TargetEmail,
		TargetPhone: invite.TargetPhone,
		MatchedUser: invite.InvitedUserID != nil,
		URL:         url,
		DeepLink:    deepLink,
		Message:     message,
		Status:      invite.Status,
		Group:       group,
		CreatedAt:   invite.CreatedAt,
	}
}

func splitInviteURL(token string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("FINNRI_PUBLIC_WEB_URL")), "/")
	if baseURL == "" {
		baseURL = "https://finnri.app"
	}
	return fmt.Sprintf("%s/invite/split/%s", baseURL, token)
}

func splitInviteDeepLink(token string) string {
	return fmt.Sprintf("ezmoney://invite/split/%s", token)
}

func displayNameForUser(user models.User) string {
	if strings.TrimSpace(user.Username) != "" {
		return strings.TrimSpace(user.Username)
	}
	if user.Email != nil && strings.TrimSpace(*user.Email) != "" {
		return strings.TrimSpace(*user.Email)
	}
	if user.Phone != nil && strings.TrimSpace(*user.Phone) != "" {
		return strings.TrimSpace(*user.Phone)
	}
	return "Finnri user"
}

func stringFromPointer(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func splitInviteUserIdentityQuery(user models.User) (string, []any) {
	parts := []string{}
	args := []any{}
	if user.Email != nil && strings.TrimSpace(*user.Email) != "" {
		parts = append(parts, "LOWER(email) = ?")
		args = append(args, strings.ToLower(strings.TrimSpace(*user.Email)))
	}
	if user.Phone != nil && strings.TrimSpace(*user.Phone) != "" {
		parts = append(parts, "phone = ?")
		args = append(args, strings.TrimSpace(*user.Phone))
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
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

func userCanAccessActiveSplitGroup(userID, groupID uint) (bool, error) {
	var count int64
	err := database.DB.Model(&models.SplitGroup{}).
		Where("id = ? AND archived = ?", groupID, false).
		Where(
			"user_id = ? OR id IN (?)",
			userID,
			database.DB.Model(&models.SplitGroupUserMember{}).
				Select("group_id").
				Where("user_id = ? AND status = ?", userID, "active"),
		).
		Count(&count).Error
	return count == 1, err
}

func activeSharedSplitGroupIDs(db *gorm.DB, userID uint) ([]uint, error) {
	var ids []uint
	err := db.Model(&models.SplitGroupUserMember{}).
		Where("user_id = ? AND status = ?", userID, "active").
		Pluck("group_id", &ids).Error
	return ids, err
}

func accessibleActiveSplitGroupIDs(db *gorm.DB, userID uint) ([]uint, error) {
	var ids []uint
	err := db.Model(&models.SplitGroup{}).
		Where("archived = ?", false).
		Where(
			"user_id = ? OR id IN (?)",
			userID,
			db.Model(&models.SplitGroupUserMember{}).
				Select("group_id").
				Where("user_id = ? AND status = ?", userID, "active"),
		).
		Pluck("id", &ids).Error
	return ids, err
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
