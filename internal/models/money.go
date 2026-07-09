package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Money stores an INR-compatible fixed-point amount in minor units. JSON is
// emitted as a number for client compatibility, while requests may use either
// a JSON number or a decimal string.
type Money int64

func ParseMoney(value string) (Money, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("amount is required")
	}
	negative := strings.HasPrefix(value, "-")
	if negative || strings.HasPrefix(value, "+") {
		value = value[1:]
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("must be a decimal amount")
	}
	if len(parts) == 2 && len(parts[1]) > 2 {
		return 0, fmt.Errorf("must have at most two decimal places")
	}
	major, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("must be a decimal amount")
	}
	minor := int64(0)
	if len(parts) == 2 {
		fraction := parts[1] + strings.Repeat("0", 2-len(parts[1]))
		if fraction != "" {
			minor, err = strconv.ParseInt(fraction, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("must be a decimal amount")
			}
		}
	}
	if major > (math.MaxInt64-minor)/100 {
		return 0, fmt.Errorf("amount is too large")
	}
	result := Money(major*100 + minor)
	if negative {
		result = -result
	}
	return result, nil
}

func (m Money) String() string {
	value := int64(m)
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	return fmt.Sprintf("%s%d.%02d", sign, value/100, value%100)
}

func (m Money) Float64() float64 {
	return float64(m) / 100
}

func (m Money) IsPositive() bool {
	return m > 0
}

func (m Money) MarshalJSON() ([]byte, error) {
	return []byte(m.String()), nil
}

func (m *Money) UnmarshalJSON(data []byte) error {
	var value string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
	} else {
		value = string(data)
	}
	parsed, err := ParseMoney(value)
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}

func (m Money) Value() (driver.Value, error) {
	return m.String(), nil
}

func (m *Money) Scan(value any) error {
	if value == nil {
		*m = 0
		return nil
	}
	var raw string
	switch typed := value.(type) {
	case []byte:
		raw = string(typed)
	case string:
		raw = typed
	case int64:
		raw = strconv.FormatInt(typed, 10)
	case float64:
		raw = strconv.FormatFloat(typed, 'f', 2, 64)
	default:
		return fmt.Errorf("unsupported money value %T", value)
	}
	parsed, err := ParseMoney(raw)
	if err != nil {
		return err
	}
	*m = parsed
	return nil
}
