package http

import "testing"

func TestParseEntryPagination(t *testing.T) {
	page, size, fields := parseEntryPagination("", "")
	if page != 1 || size != 50 || len(fields) != 0 {
		t.Fatalf("unexpected defaults: page=%d size=%d fields=%v", page, size, fields)
	}

	page, size, fields = parseEntryPagination("2", "100")
	if page != 2 || size != 100 || len(fields) != 0 {
		t.Fatalf("unexpected parsed values: page=%d size=%d fields=%v", page, size, fields)
	}

	_, _, fields = parseEntryPagination("0", "101")
	if fields["page"] == "" || fields["page_size"] == "" {
		t.Fatalf("expected stable field errors, got %v", fields)
	}
}

func TestParseIdempotencyKey(t *testing.T) {
	key, fields := parseIdempotencyKey("  retry-123  ")
	if key != "retry-123" || len(fields) != 0 {
		t.Fatalf("unexpected key result: key=%q fields=%v", key, fields)
	}

	_, fields = parseIdempotencyKey(string(make([]byte, 129)))
	if fields["idempotency_key"] == "" {
		t.Fatalf("expected length error, got %v", fields)
	}
}
