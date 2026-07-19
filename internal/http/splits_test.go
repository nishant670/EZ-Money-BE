package http

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

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
