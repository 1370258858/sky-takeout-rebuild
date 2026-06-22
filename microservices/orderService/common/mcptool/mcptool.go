package common

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// FindValue locates a value by key with loose matching.
// It supports common key styles like userId / user_id / USER-ID.
func FindValue(content map[string]any, keys ...string) (any, bool) {
	if len(content) == 0 || len(keys) == 0 {
		return nil, false
	}

	for _, k := range keys {
		if v, ok := content[k]; ok {
			return v, true
		}
	}

	normalized := make(map[string]any, len(content))
	for k, v := range content {
		normalized[normalizeKey(k)] = v
	}

	for _, k := range keys {
		if v, ok := normalized[normalizeKey(k)]; ok {
			return v, true
		}
	}

	return nil, false
}

// MustUint64 reads a required uint64 field from map content.
func MustUint64(content map[string]any, keys ...string) (uint64, error) {
	raw, ok := FindValue(content, keys...)
	if !ok {
		return 0, fmt.Errorf("required field missing: %s", strings.Join(keys, "/"))
	}

	uid, err := ToUint64(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid field %s: %w", strings.Join(keys, "/"), err)
	}

	if uid == 0 {
		return 0, fmt.Errorf("invalid field %s: must be greater than 0", strings.Join(keys, "/"))
	}

	return uid, nil
}

// ToUint64 converts common numeric and string values to uint64.
func ToUint64(v any) (uint64, error) {
	switch n := v.(type) {
	case uint64:
		return n, nil
	case uint32:
		return uint64(n), nil
	case uint16:
		return uint64(n), nil
	case uint8:
		return uint64(n), nil
	case uint:
		return uint64(n), nil
	case int:
		if n < 0 {
			return 0, fmt.Errorf("negative value")
		}
		return uint64(n), nil
	case int64:
		if n < 0 {
			return 0, fmt.Errorf("negative value")
		}
		return uint64(n), nil
	case int32:
		if n < 0 {
			return 0, fmt.Errorf("negative value")
		}
		return uint64(n), nil
	case int16:
		if n < 0 {
			return 0, fmt.Errorf("negative value")
		}
		return uint64(n), nil
	case int8:
		if n < 0 {
			return 0, fmt.Errorf("negative value")
		}
		return uint64(n), nil
	case float64:
		if n < 0 {
			return 0, fmt.Errorf("negative value")
		}
		return uint64(n), nil
	case float32:
		if n < 0 {
			return 0, fmt.Errorf("negative value")
		}
		return uint64(n), nil
	case json.Number:
		u, err := strconv.ParseUint(n.String(), 10, 64)
		if err == nil {
			return u, nil
		}
		f, ferr := strconv.ParseFloat(n.String(), 64)
		if ferr != nil || f < 0 {
			return 0, fmt.Errorf("cannot parse %q", n.String())
		}
		return uint64(f), nil
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return 0, fmt.Errorf("empty string")
		}
		u, err := strconv.ParseUint(s, 10, 64)
		if err == nil {
			return u, nil
		}
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil || f < 0 {
			return 0, fmt.Errorf("cannot parse %q", s)
		}
		return uint64(f), nil
	default:
		return 0, fmt.Errorf("unsupported type %T", v)
	}
}

func normalizeKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}
