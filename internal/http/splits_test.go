package http

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"finance-parser-go/internal/database"
	"finance-parser-go/internal/models"
)

func TestSplitLedgerFlowComputesFriendBalances(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useSmokeDatabase(t)

	router := smokeRouter(t)
	_, token := createPaidBillingTestUserSession(t)

	friend := performJSONRequest[models.SplitFriend](
		t, router, http.MethodPost, "/v1/split/friends", token,
		map[string]any{"name": "Aarav", "phone": "+919999999999"}, http.StatusCreated,
	)
	if friend.ID == 0 || friend.Name != "Aarav" {
		t.Fatalf("unexpected split friend: %#v", friend)
	}

	bill := performJSONRequest[models.SplitBill](
		t, router, http.MethodPost, "/v1/split/bills", token,
		map[string]any{
			"title":        "Dinner",
			"total_amount": "1000.00",
			"currency":     "INR",
			"date":         "2026-07-12",
			"participants": []map[string]any{
				{
					"friend_id":    friend.ID,
					"share_amount": "400.00",
					"direction":    splitDirectionFriendOwesUser,
				},
			},
		}, http.StatusCreated,
	)
	if bill.ID == 0 || len(bill.Participants) != 1 {
		t.Fatalf("unexpected split bill: %#v", bill)
	}

	settlement := performJSONRequest[models.SplitSettlement](
		t, router, http.MethodPost, "/v1/split/settlements", token,
		map[string]any{
			"friend_id": friend.ID,
			"amount":    "150.00",
			"direction": settlementDirectionFriendPaidUser,
			"date":      "2026-07-13",
		}, http.StatusCreated,
	)
	if settlement.ID == 0 || settlement.FriendID != friend.ID {
		t.Fatalf("unexpected split settlement: %#v", settlement)
	}

	balances := performJSONRequest[[]splitBalance](
		t, router, http.MethodGet, "/v1/split/balances", token, nil, http.StatusOK,
	)
	if len(balances) != 1 {
		t.Fatalf("expected one split balance, got %#v", balances)
	}
	if balances[0].TotalOwedByFriend.String() != "250.00" || balances[0].NetBalance.String() != "250.00" {
		t.Fatalf("unexpected split balance: %#v", balances[0])
	}

	activity := performJSONRequest[struct {
		Items []splitActivityItem `json:"items"`
	}](
		t, router, http.MethodGet, "/v1/split/activity", token, nil, http.StatusOK,
	)
	if len(activity.Items) < 3 {
		t.Fatalf("expected split activity for friend, bill, and settlement, got %#v", activity.Items)
	}
	seenTypes := map[string]bool{}
	for _, item := range activity.Items {
		seenTypes[item.Type] = true
	}
	for _, activityType := range []string{"friend_created", "bill", "settlement"} {
		if !seenTypes[activityType] {
			t.Fatalf("expected activity type %s in %#v", activityType, activity.Items)
		}
	}
}

func TestSplitGroupCanBeCreatedBeforeMembers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useSmokeDatabase(t)

	router := smokeRouter(t)
	_, token := createPaidBillingTestUserSession(t)

	group := performJSONRequest[models.SplitGroup](
		t, router, http.MethodPost, "/v1/split/groups", token,
		map[string]any{"name": "Bubu Dudu", "friend_ids": []uint{}}, http.StatusCreated,
	)
	if group.ID == 0 || group.Name != "Bubu Dudu" || len(group.Members) != 0 {
		t.Fatalf("expected empty split group to be created, got %#v", group)
	}
	activity := performJSONRequest[struct {
		Items []splitActivityItem `json:"items"`
	}](
		t, router, http.MethodGet, "/v1/split/activity", token, nil, http.StatusOK,
	)
	if len(activity.Items) == 0 || activity.Items[0].Type != "group_created" || activity.Items[0].Group == nil {
		t.Fatalf("expected group creation activity, got %#v", activity.Items)
	}
}

func TestSplitGroupInviteLinkCanBeCreatedAndReused(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useSmokeDatabase(t)

	router := smokeRouter(t)
	_, token := createPaidBillingTestUserSession(t)

	group := performJSONRequest[models.SplitGroup](
		t, router, http.MethodPost, "/v1/split/groups", token,
		map[string]any{"name": "Bhaukaal", "friend_ids": []uint{}}, http.StatusCreated,
	)

	firstInvite := performJSONRequest[splitGroupInviteResponse](
		t, router, http.MethodPost, fmt.Sprintf("/v1/split/groups/%d/invite-link", group.ID), token,
		nil, http.StatusOK,
	)
	if firstInvite.Token == "" || firstInvite.URL == "" || firstInvite.DeepLink == "" {
		t.Fatalf("expected invite link response, got %#v", firstInvite)
	}
	if firstInvite.Group.ID != group.ID {
		t.Fatalf("expected invite group %d, got %#v", group.ID, firstInvite.Group)
	}

	secondInvite := performJSONRequest[splitGroupInviteResponse](
		t, router, http.MethodPost, fmt.Sprintf("/v1/split/groups/%d/invite-link", group.ID), token,
		nil, http.StatusOK,
	)
	if secondInvite.Token != firstInvite.Token {
		t.Fatalf("expected active invite to be reused, first=%q second=%q", firstInvite.Token, secondInvite.Token)
	}
}

func TestSplitGroupInviteCanBeViewedAndAccepted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useSmokeDatabase(t)

	router := smokeRouter(t)
	owner, ownerToken := createPaidBillingTestUserSession(t)
	guest, guestToken := createPaidBillingTestUserSession(t)
	email := "guest@example.com"
	if err := database.DB.Model(&guest).Update("email", email).Error; err != nil {
		t.Fatalf("failed to set guest email: %v", err)
	}

	group := performJSONRequest[models.SplitGroup](
		t, router, http.MethodPost, "/v1/split/groups", ownerToken,
		map[string]any{"name": "Bhaukaal", "friend_ids": []uint{}}, http.StatusCreated,
	)
	directInvite := performJSONRequest[splitGroupDirectInviteResponse](
		t, router, http.MethodPost, fmt.Sprintf("/v1/split/groups/%d/invites", group.ID), ownerToken,
		map[string]any{"email": email}, http.StatusCreated,
	)
	if directInvite.ID == 0 || !directInvite.MatchedUser || !directInvite.NotificationSent || directInvite.URL == "" || directInvite.Message == "" {
		t.Fatalf("expected matched direct invite response, got %#v", directInvite)
	}
	duplicateDirectInvite := performJSONRequest[splitGroupDirectInviteResponse](
		t, router, http.MethodPost, fmt.Sprintf("/v1/split/groups/%d/invites", group.ID), ownerToken,
		map[string]any{"email": email}, http.StatusCreated,
	)
	if duplicateDirectInvite.ID != directInvite.ID {
		t.Fatalf("expected duplicate direct invite to be reused, first=%d second=%d", directInvite.ID, duplicateDirectInvite.ID)
	}
	pendingInvites := performJSONRequest[[]splitGroupDirectInviteResponse](
		t, router, http.MethodGet, fmt.Sprintf("/v1/split/groups/%d/invites", group.ID), ownerToken,
		nil, http.StatusOK,
	)
	if len(pendingInvites) != 1 || pendingInvites[0].ID != directInvite.ID || !pendingInvites[0].MatchedUser {
		t.Fatalf("expected one pending direct invite, got %#v", pendingInvites)
	}
	if pendingInvites[0].URL == "" || pendingInvites[0].Message == "" {
		t.Fatalf("expected pending invite to include share details, got %#v", pendingInvites[0])
	}
	performJSONRequest[map[string]string](
		t, router, http.MethodGet, fmt.Sprintf("/v1/split/groups/%d/invites", group.ID), guestToken,
		nil, http.StatusNotFound,
	)
	performJSONRequest[map[string]string](
		t, router, http.MethodDelete, fmt.Sprintf("/v1/split/groups/%d/invites/%d", group.ID, directInvite.ID), guestToken,
		nil, http.StatusNotFound,
	)
	performJSONRequest[map[string]string](
		t, router, http.MethodDelete, fmt.Sprintf("/v1/split/groups/%d/invites/%d", group.ID, directInvite.ID), ownerToken,
		nil, http.StatusOK,
	)
	pendingInvitesAfterRevoke := performJSONRequest[[]splitGroupDirectInviteResponse](
		t, router, http.MethodGet, fmt.Sprintf("/v1/split/groups/%d/invites", group.ID), ownerToken,
		nil, http.StatusOK,
	)
	if len(pendingInvitesAfterRevoke) != 0 {
		t.Fatalf("expected revoked direct invite to leave pending list, got %#v", pendingInvitesAfterRevoke)
	}
	var receivedNotification models.Notification
	if err := database.DB.Where("user_id = ? AND type = ?", guest.ID, "split.group_invite.received").
		First(&receivedNotification).Error; err != nil {
		t.Fatalf("expected guest invite notification: %v", err)
	}

	invite := performJSONRequest[splitGroupInviteResponse](
		t, router, http.MethodPost, fmt.Sprintf("/v1/split/groups/%d/invite-link", group.ID), ownerToken,
		nil, http.StatusOK,
	)
	if invite.URL != directInvite.URL {
		t.Fatalf("expected direct invite to use active group invite link, direct=%q link=%q", directInvite.URL, invite.URL)
	}

	details := performJSONRequest[splitGroupInviteDetailsResponse](
		t, router, http.MethodGet, fmt.Sprintf("/v1/split/invites/%s", invite.Token), guestToken,
		nil, http.StatusOK,
	)
	if details.Group.ID != group.ID || details.OwnerName == "" {
		t.Fatalf("unexpected invite details: %#v", details)
	}

	accepted := performJSONRequest[splitGroupInviteAcceptResponse](
		t, router, http.MethodPost, fmt.Sprintf("/v1/split/invites/%s/accept", invite.Token), guestToken,
		nil, http.StatusOK,
	)
	if accepted.Group.ID != group.ID || accepted.Friend.ID == 0 || accepted.Member.ID == 0 {
		t.Fatalf("unexpected invite accept response: %#v", accepted)
	}
	if accepted.Friend.UserID != owner.ID || accepted.Friend.Email != email {
		t.Fatalf("expected owner-scoped friend for invitee, got %#v", accepted.Friend)
	}

	guestGroups := performJSONRequest[[]models.SplitGroup](
		t, router, http.MethodGet, "/v1/split/groups", guestToken,
		nil, http.StatusOK,
	)
	guestCanSeeGroup := false
	for _, guestGroup := range guestGroups {
		if guestGroup.ID == group.ID {
			if guestGroup.ViewerRole != "member" || guestGroup.ViewerCanManage || !guestGroup.ViewerCanAddExpense {
				t.Fatalf("expected guest group member permissions, got %#v", guestGroup)
			}
			guestCanSeeGroup = true
			break
		}
	}
	if !guestCanSeeGroup {
		t.Fatalf("expected guest to see accepted group, got %#v", guestGroups)
	}

	bill := performJSONRequest[models.SplitBill](
		t, router, http.MethodPost, "/v1/split/bills", ownerToken,
		map[string]any{
			"group_id":     group.ID,
			"title":        "Dinner",
			"total_amount": "1200.00",
			"currency":     "INR",
			"date":         "2026-07-27",
			"participants": []map[string]any{
				{
					"friend_id":    accepted.Friend.ID,
					"share_amount": "600.00",
					"direction":    splitDirectionFriendOwesUser,
				},
			},
		}, http.StatusCreated,
	)
	guestBills := performJSONRequest[[]models.SplitBill](
		t, router, http.MethodGet, "/v1/split/bills", guestToken,
		nil, http.StatusOK,
	)
	guestCanSeeBill := false
	for _, guestBill := range guestBills {
		if guestBill.ID == bill.ID && guestBill.GroupID != nil && *guestBill.GroupID == group.ID {
			if guestBill.ViewerCanEdit || guestBill.ViewerCanDelete {
				t.Fatalf("expected guest not to edit owner-created bill, got %#v", guestBill)
			}
			guestCanSeeBill = true
			break
		}
	}
	if !guestCanSeeBill {
		t.Fatalf("expected guest to see shared group bill, got %#v", guestBills)
	}

	guestBill := performJSONRequest[models.SplitBill](
		t, router, http.MethodPost, "/v1/split/bills", guestToken,
		map[string]any{
			"group_id":     group.ID,
			"title":        "Snacks",
			"total_amount": "400.00",
			"currency":     "INR",
			"date":         "2026-07-28",
			"participants": []map[string]any{
				{
					"friend_id":    accepted.Friend.ID,
					"share_amount": "200.00",
					"direction":    splitDirectionFriendOwesUser,
				},
			},
		}, http.StatusCreated,
	)
	if guestBill.UserID != guest.ID || guestBill.GroupID == nil || *guestBill.GroupID != group.ID {
		t.Fatalf("expected guest-created bill in shared group, got %#v", guestBill)
	}
	if !guestBill.ViewerCanEdit || !guestBill.ViewerCanDelete {
		t.Fatalf("expected guest to manage own bill, got %#v", guestBill)
	}

	ownerBills := performJSONRequest[[]models.SplitBill](
		t, router, http.MethodGet, "/v1/split/bills", ownerToken,
		nil, http.StatusOK,
	)
	ownerCanSeeGuestBill := false
	for _, ownerBill := range ownerBills {
		if ownerBill.ID == guestBill.ID && ownerBill.GroupID != nil && *ownerBill.GroupID == group.ID {
			if ownerBill.ViewerCanEdit || ownerBill.ViewerCanDelete {
				t.Fatalf("expected owner not to edit guest-created bill, got %#v", ownerBill)
			}
			ownerCanSeeGuestBill = true
			break
		}
	}
	if !ownerCanSeeGuestBill {
		t.Fatalf("expected owner to see guest-created group bill, got %#v", ownerBills)
	}

	performJSONRequest[map[string]string](
		t, router, http.MethodPost, fmt.Sprintf("/v1/split/groups/%d/leave", group.ID), guestToken,
		nil, http.StatusOK,
	)

	guestGroupsAfterLeave := performJSONRequest[[]models.SplitGroup](
		t, router, http.MethodGet, "/v1/split/groups", guestToken,
		nil, http.StatusOK,
	)
	for _, guestGroup := range guestGroupsAfterLeave {
		if guestGroup.ID == group.ID {
			t.Fatalf("expected guest group to disappear after leaving, got %#v", guestGroupsAfterLeave)
		}
	}

	guestBillsAfterLeave := performJSONRequest[[]models.SplitBill](
		t, router, http.MethodGet, "/v1/split/bills", guestToken,
		nil, http.StatusOK,
	)
	for _, guestBillAfterLeave := range guestBillsAfterLeave {
		if guestBillAfterLeave.GroupID != nil && *guestBillAfterLeave.GroupID == group.ID {
			t.Fatalf("expected shared group bills to disappear after leaving, got %#v", guestBillsAfterLeave)
		}
	}

	ownerGroupsAfterLeave := performJSONRequest[[]models.SplitGroup](
		t, router, http.MethodGet, "/v1/split/groups", ownerToken,
		nil, http.StatusOK,
	)
	ownerStillHasGroup := false
	for _, ownerGroup := range ownerGroupsAfterLeave {
		if ownerGroup.ID == group.ID {
			ownerStillHasGroup = true
			break
		}
	}
	if !ownerStillHasGroup {
		t.Fatalf("expected owner to keep group after guest leaves, got %#v", ownerGroupsAfterLeave)
	}

	performJSONRequest[map[string]string](
		t, router, http.MethodPost, fmt.Sprintf("/v1/split/groups/%d/leave", group.ID), ownerToken,
		nil, http.StatusUnprocessableEntity,
	)

	var notification models.Notification
	if err := database.DB.Where("user_id = ? AND type = ?", owner.ID, "split.group_invite.accepted").
		First(&notification).Error; err != nil {
		t.Fatalf("expected owner notification: %v", err)
	}
}

func TestSplitBillInputRejectsInvalidShares(t *testing.T) {
	amount, _ := models.ParseMoney("100.00")
	share, _ := models.ParseMoney("70.00")
	input := splitBillInput{
		Title:       "Cab",
		TotalAmount: amount,
		Currency:    "INR",
		Date:        "2026-07-12",
		Participants: []splitParticipantInput{
			{FriendID: 1, ShareAmount: share, Direction: splitDirectionFriendOwesUser},
			{FriendID: 2, ShareAmount: share, Direction: splitDirectionFriendOwesUser},
		},
	}

	fields := input.validate()
	if fields["participants"] == "" {
		t.Fatalf("expected shares over total to be rejected, got %v", fields)
	}
}

func TestEntryCreateCanCreateLinkedSplitWithInlineFriend(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useSmokeDatabase(t)

	router := smokeRouter(t)
	_, token := createPaidBillingTestUserSession(t)
	accounts := performJSONRequest[[]models.Account](
		t, router, http.MethodGet, "/v1/accounts", token, nil, http.StatusOK,
	)
	if len(accounts) == 0 {
		t.Fatal("guest account was not created")
	}

	entry := performJSONRequest[models.Entry](
		t, router, http.MethodPost, "/v1/entries", token,
		map[string]any{
			"title":      "Trip dinner",
			"type":       "expense",
			"amount":     "2000.00",
			"currency":   "INR",
			"source":     "manual",
			"mode":       "UPI",
			"category":   "Food",
			"merchant":   "Cafe",
			"date":       "2026-07-13",
			"time":       "21:00",
			"account_id": accounts[0].ID,
			"split": map[string]any{
				"group_name": "Goa trip",
				"participants": []map[string]any{
					{
						"friend":       map[string]any{"name": "Riya"},
						"share_amount": "1000.00",
					},
				},
			},
		}, http.StatusCreated,
	)
	if entry.ID == 0 {
		t.Fatalf("entry was not created: %#v", entry)
	}

	bills := performJSONRequest[[]models.SplitBill](
		t, router, http.MethodGet, "/v1/split/bills", token, nil, http.StatusOK,
	)
	if len(bills) != 1 || bills[0].EntryID == nil || *bills[0].EntryID != entry.ID {
		t.Fatalf("expected one linked split bill, got %#v", bills)
	}
	if bills[0].GroupID == nil || len(bills[0].Participants) != 1 {
		t.Fatalf("expected linked group and participant, got %#v", bills[0])
	}

	balances := performJSONRequest[[]splitBalance](
		t, router, http.MethodGet, "/v1/split/balances", token, nil, http.StatusOK,
	)
	if len(balances) != 1 || balances[0].NetBalance.String() != "1000.00" {
		t.Fatalf("expected friend to owe 1000.00, got %#v", balances)
	}

	updatedBill := performJSONRequest[models.SplitBill](
		t, router, http.MethodPut, fmt.Sprintf("/v1/split/bills/%d", bills[0].ID), token,
		map[string]any{
			"entry_id":     entry.ID,
			"group_id":     *bills[0].GroupID,
			"title":        "Trip dinner",
			"total_amount": "2000.00",
			"currency":     "INR",
			"date":         "2026-07-13",
			"participants": []map[string]any{
				{
					"friend_id":    balances[0].Friend.ID,
					"share_amount": "750.00",
					"direction":    splitDirectionFriendOwesUser,
				},
			},
		}, http.StatusOK,
	)
	if len(updatedBill.Participants) != 1 || updatedBill.Participants[0].ShareAmount.String() != "750.00" {
		t.Fatalf("expected updated participant share, got %#v", updatedBill.Participants)
	}

	performJSONRequest[map[string]string](
		t, router, http.MethodDelete, fmt.Sprintf("/v1/split/bills/%d", bills[0].ID), token, nil, http.StatusOK,
	)
	bills = performJSONRequest[[]models.SplitBill](
		t, router, http.MethodGet, "/v1/split/bills", token, nil, http.StatusOK,
	)
	if len(bills) != 0 {
		t.Fatalf("expected linked split bill to be deleted, got %#v", bills)
	}
}

func TestEntryUpdateAndDeleteManageLinkedSplitAtomically(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useSmokeDatabase(t)

	router := smokeRouter(t)
	_, token := createPaidBillingTestUserSession(t)
	accounts := performJSONRequest[[]models.Account](
		t, router, http.MethodGet, "/v1/accounts", token, nil, http.StatusOK,
	)
	if len(accounts) == 0 {
		t.Fatal("guest account was not created")
	}

	entryPayload := map[string]any{
		"title":      "Dinner",
		"type":       "expense",
		"amount":     "1800.00",
		"currency":   "INR",
		"source":     "manual",
		"mode":       "UPI",
		"category":   "Food",
		"merchant":   "Cafe",
		"date":       "2026-07-14",
		"time":       "20:00",
		"account_id": accounts[0].ID,
	}
	entry := performJSONRequest[models.Entry](
		t, router, http.MethodPost, "/v1/entries", token, entryPayload, http.StatusCreated,
	)

	entryPayload["split"] = map[string]any{
		"group_name": "Dinner crew",
		"participants": []map[string]any{
			{
				"friend":       map[string]any{"name": "Kabir"},
				"share_amount": "900.00",
				"direction":    splitDirectionFriendOwesUser,
			},
		},
	}
	performJSONRequest[models.Entry](
		t, router, http.MethodPut, fmt.Sprintf("/v1/entries/%d", entry.ID), token, entryPayload, http.StatusOK,
	)
	bills := performJSONRequest[[]models.SplitBill](
		t, router, http.MethodGet, "/v1/split/bills", token, nil, http.StatusOK,
	)
	if len(bills) != 1 || bills[0].EntryID == nil || *bills[0].EntryID != entry.ID {
		t.Fatalf("expected update to create linked split bill, got %#v", bills)
	}

	entryPayload["split"] = nil
	performJSONRequest[models.Entry](
		t, router, http.MethodPut, fmt.Sprintf("/v1/entries/%d", entry.ID), token, entryPayload, http.StatusOK,
	)
	bills = performJSONRequest[[]models.SplitBill](
		t, router, http.MethodGet, "/v1/split/bills", token, nil, http.StatusOK,
	)
	if len(bills) != 0 {
		t.Fatalf("expected split:null to remove linked split bill, got %#v", bills)
	}

	entryPayload["split"] = map[string]any{
		"participants": []map[string]any{
			{
				"friend":       map[string]any{"name": "Kabir"},
				"share_amount": "600.00",
				"direction":    splitDirectionFriendOwesUser,
			},
		},
	}
	performJSONRequest[models.Entry](
		t, router, http.MethodPut, fmt.Sprintf("/v1/entries/%d", entry.ID), token, entryPayload, http.StatusOK,
	)
	performJSONRequest[map[string]string](
		t, router, http.MethodDelete, fmt.Sprintf("/v1/entries/%d", entry.ID), token, nil, http.StatusOK,
	)
	bills = performJSONRequest[[]models.SplitBill](
		t, router, http.MethodGet, "/v1/split/bills", token, nil, http.StatusOK,
	)
	if len(bills) != 0 {
		t.Fatalf("expected deleting entry to remove linked split bill, got %#v", bills)
	}
}
