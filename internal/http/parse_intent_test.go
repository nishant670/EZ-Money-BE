package http

import "testing"

// The fragments people actually type, and the sentences they do not.
//
// Every case here is a real shape from the capture field rather than a
// permutation of the vocabulary: the point of the backstop is the two-word
// question with no verb and no question mark, which is exactly what the prompt
// calls capture by default.
func TestLedgerQuestionShape(t *testing.T) {
	cases := []struct {
		name       string
		transcript string
		wantOK     bool
		wantMetric string
		wantPeriod string
	}{
		{
			// The reported bug. No question mark, no verb, no amount.
			name:       "bare period and metric",
			transcript: "today spend",
			wantOK:     true,
			wantMetric: metricSpendTotal,
			wantPeriod: "today",
		},
		{
			name:       "the same words with a question mark",
			transcript: "today spends?",
			wantOK:     true,
			wantMetric: metricSpendTotal,
			wantPeriod: "today",
		},
		{
			name:       "metric with no period at all",
			transcript: "food spends",
			wantOK:     true,
			wantMetric: metricSpendTotal,
			// Left absent on purpose: the resolver owns the "this month"
			// default, and the card states it.
			wantPeriod: "",
		},
		{
			name:       "superlative",
			transcript: "biggest spend last week",
			wantOK:     true,
			wantMetric: metricLargest,
			wantPeriod: "last_week",
		},
		{
			name:       "where the money went",
			transcript: "where did my money go last month",
			wantOK:     true,
			wantMetric: metricBreakdown,
			wantPeriod: "last_month",
		},
		{
			name:       "income side",
			transcript: "income this year",
			wantOK:     true,
			wantMetric: metricIncomeTotal,
			wantPeriod: "this_year",
		},
		{
			name:       "a period whose own name carries digits",
			transcript: "spend last 30 days",
			wantOK:     true,
			wantMetric: metricSpendTotal,
			wantPeriod: "last_30_days",
		},
		{
			// The guard that matters most. An amount in the text means money
			// moved, whatever the surrounding words look like.
			name:       "a capture that uses a metric word",
			transcript: "spent 250 on food via UPI",
			wantOK:     false,
		},
		{
			name:       "a capture with no amount and no metric word",
			transcript: "paid rent to landlord",
			wantOK:     false,
		},
		{
			name:       "chatter",
			transcript: "hello there",
			wantOK:     false,
		},
		{
			name:       "empty",
			transcript: "   ",
			wantOK:     false,
		},
		{
			// Word boundaries: a substring match would read "spend" here.
			name:       "a word that merely contains a metric word",
			transcript: "monthly stipend",
			wantOK:     false,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			query, ok := ledgerQuestionShape(testCase.transcript)
			if ok != testCase.wantOK {
				t.Fatalf("ledgerQuestionShape(%q) ok = %v, want %v", testCase.transcript, ok, testCase.wantOK)
			}
			if !testCase.wantOK {
				return
			}
			if metric, _ := query["metric"].(string); metric != testCase.wantMetric {
				t.Errorf("metric = %q, want %q", metric, testCase.wantMetric)
			}
			period, hasPeriod := query["period"].(map[string]any)
			if testCase.wantPeriod == "" {
				if hasPeriod {
					t.Errorf("period = %v, want it left to the resolver's default", period)
				}
				return
			}
			if !hasPeriod {
				t.Fatalf("period missing, want kind %q", testCase.wantPeriod)
			}
			if kind, _ := period["kind"].(string); kind != testCase.wantPeriod {
				t.Errorf("period kind = %q, want %q", kind, testCase.wantPeriod)
			}
		})
	}
}

// The backstop converts in one direction only, and never over a capture signal.
func TestParsedQuestionQuery(t *testing.T) {
	t.Run("the model's own question is taken as read", func(t *testing.T) {
		own := map[string]any{"metric": metricCount}
		entry := map[string]any{"intent": "question", "query": own}

		query, ok := parsedQuestionQuery(entry, "anything at all")
		if !ok {
			t.Fatal("a question the model routed itself must stay a question")
		}
		// Handed straight through — the backstop must not rewrite a query the
		// model built from the full sentence.
		if got, _ := query.(map[string]any); got == nil || got["metric"] != metricCount {
			t.Errorf("query = %v, want the model's own", query)
		}
	})

	t.Run("a capture with no amount and a question shape flips", func(t *testing.T) {
		entry := map[string]any{"intent": "capture", "amount": nil, "category": "Misc"}

		query, ok := parsedQuestionQuery(entry, "today spend")
		if !ok {
			t.Fatal("want the backstop to route this to the answer side")
		}
		if got, _ := query.(map[string]any); got == nil || got["metric"] != metricSpendTotal {
			t.Errorf("query = %v, want a spend total", query)
		}
	})

	t.Run("a capture that names an amount never flips", func(t *testing.T) {
		// The safety argument in one case: whatever the wording, an amount
		// means money moved and the user is owed a draft.
		entry := map[string]any{"intent": "capture", "amount": float64(250)}

		if _, ok := parsedQuestionQuery(entry, "today spend 250"); ok {
			t.Fatal("a draft carrying an amount must stay a capture")
		}
	})

	t.Run("a split candidate outweighs the wording", func(t *testing.T) {
		entry := map[string]any{"intent": "capture", "split_candidate": true}

		if _, ok := parsedQuestionQuery(entry, "dinner spend split with Ria"); ok {
			t.Fatal("a split is a claim about money that moved")
		}
	})

	t.Run("a subscription candidate outweighs the wording", func(t *testing.T) {
		entry := map[string]any{
			"intent":                 "capture",
			"subscription_candidate": map[string]any{"name": "Netflix"},
		}

		if _, ok := parsedQuestionQuery(entry, "netflix monthly spend"); ok {
			t.Fatal("a subscription is a claim about money that moved")
		}
	})

	t.Run("an ordinary capture is left alone", func(t *testing.T) {
		entry := map[string]any{"intent": "capture", "amount": nil, "merchant": "Landlord"}

		if _, ok := parsedQuestionQuery(entry, "paid rent to landlord"); ok {
			t.Fatal("nothing here asks about records")
		}
	})
}
