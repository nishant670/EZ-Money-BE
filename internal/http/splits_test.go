package http

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"finance-parser-go/internal/models"
)

func TestSplitLedgerFlowComputesFriendBalances(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useSmokeDatabase(t)

	router := smokeRouter(t)
	authResponse := performJSONRequest[AuthResponse](
		t, router, http.MethodPost, "/v1/auth/guest", "", map[string]string{
			"device_id": "split-ledger-device",
		}, http.StatusOK,
	)
	if !strings.HasPrefix(authResponse.Token, "fnr_") {
		t.Fatalf("expected opaque guest session token, got %q", authResponse.Token)
	}

	friend := performJSONRequest[models.SplitFriend](
		t, router, http.MethodPost, "/v1/split/friends", authResponse.Token,
		map[string]any{"name": "Aarav", "phone": "+919999999999"}, http.StatusCreated,
	)
	if friend.ID == 0 || friend.Name != "Aarav" {
		t.Fatalf("unexpected split friend: %#v", friend)
	}

	bill := performJSONRequest[models.SplitBill](
		t, router, http.MethodPost, "/v1/split/bills", authResponse.Token,
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
		t, router, http.MethodPost, "/v1/split/settlements", authResponse.Token,
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
		t, router, http.MethodGet, "/v1/split/balances", authResponse.Token, nil, http.StatusOK,
	)
	if len(balances) != 1 {
		t.Fatalf("expected one split balance, got %#v", balances)
	}
	if balances[0].TotalOwedByFriend.String() != "250.00" || balances[0].NetBalance.String() != "250.00" {
		t.Fatalf("unexpected split balance: %#v", balances[0])
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
