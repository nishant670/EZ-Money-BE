package http

import (
	"strconv"
	"strings"
)

const (
	defaultEntryPageSize = 50
	maxEntryPageSize     = 100
)

func parseEntryPagination(rawPage, rawPageSize string) (int, int, map[string]string) {
	fields := map[string]string{}
	page := 1
	pageSize := defaultEntryPageSize

	if rawPage != "" {
		parsed, err := strconv.Atoi(rawPage)
		if err != nil || parsed < 1 {
			fields["page"] = "must be a positive integer"
		} else {
			page = parsed
		}
	}
	if rawPageSize != "" {
		parsed, err := strconv.Atoi(rawPageSize)
		if err != nil || parsed < 1 || parsed > maxEntryPageSize {
			fields["page_size"] = "must be between 1 and 100"
		} else {
			pageSize = parsed
		}
	}
	return page, pageSize, fields
}

func parseIdempotencyKey(raw string) (string, map[string]string) {
	key := strings.TrimSpace(raw)
	fields := map[string]string{}
	if len(key) > 128 {
		fields["idempotency_key"] = "must not exceed 128 characters"
	}
	return key, fields
}
