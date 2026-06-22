package mcpCommon

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// BindParamsJSON converts map input to a typed struct through json marshal/unmarshal.
func BindParamsJSON[T any](input map[string]any) (T, error) {
	var out T
	b, err := json.Marshal(input)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, err
	}
	return out, nil
}

// GetParam returns the first matched value from key or aliases.
func GetParam(input map[string]any, key string, aliases ...string) (any, bool) {
	if input == nil {
		return nil, false
	}
	if v, ok := input[key]; ok {
		return v, true
	}
	for _, alias := range aliases {
		if v, ok := input[alias]; ok {
			return v, true
		}
	}
	return nil, false
}

func GetStringParam(input map[string]any, key string, aliases ...string) (string, bool, error) {
	v, ok := GetParam(input, key, aliases...)
	if !ok {
		return "", false, nil
	}
	s, err := toString(v)
	if err != nil {
		return "", true, fmt.Errorf("%s: %w", key, err)
	}
	return s, true, nil
}

func GetIntParam(input map[string]any, key string, aliases ...string) (int, bool, error) {
	v, ok := GetParam(input, key, aliases...)
	if !ok {
		return 0, false, nil
	}
	n, err := toInt(v)
	if err != nil {
		return 0, true, fmt.Errorf("%s: %w", key, err)
	}
	return n, true, nil
}

func GetUint64Param(input map[string]any, key string, aliases ...string) (uint64, bool, error) {
	v, ok := GetParam(input, key, aliases...)
	if !ok {
		return 0, false, nil
	}
	n, err := toUint64(v)
	if err != nil {
		return 0, true, fmt.Errorf("%s: %w", key, err)
	}
	return n, true, nil
}

func GetFloat64Param(input map[string]any, key string, aliases ...string) (float64, bool, error) {
	v, ok := GetParam(input, key, aliases...)
	if !ok {
		return 0, false, nil
	}
	n, err := toFloat64(v)
	if err != nil {
		return 0, true, fmt.Errorf("%s: %w", key, err)
	}
	return n, true, nil
}

func GetBoolParam(input map[string]any, key string, aliases ...string) (bool, bool, error) {
	v, ok := GetParam(input, key, aliases...)
	if !ok {
		return false, false, nil
	}
	b, err := toBool(v)
	if err != nil {
		return false, true, fmt.Errorf("%s: %w", key, err)
	}
	return b, true, nil
}

func toString(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case fmt.Stringer:
		return x.String(), nil
	case []byte:
		return string(x), nil
	case int:
		return strconv.Itoa(x), nil
	case int8:
		return strconv.FormatInt(int64(x), 10), nil
	case int16:
		return strconv.FormatInt(int64(x), 10), nil
	case int32:
		return strconv.FormatInt(int64(x), 10), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case uint:
		return strconv.FormatUint(uint64(x), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(x), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(x), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(x), 10), nil
	case uint64:
		return strconv.FormatUint(x, 10), nil
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(x), nil
	default:
		return "", fmt.Errorf("cannot convert %T to string", v)
	}
}

func toInt(v any) (int, error) {
	switch x := v.(type) {
	case int:
		return x, nil
	case int8:
		return int(x), nil
	case int16:
		return int(x), nil
	case int32:
		return int(x), nil
	case int64:
		return int(x), nil
	case uint:
		return int(x), nil
	case uint8:
		return int(x), nil
	case uint16:
		return int(x), nil
	case uint32:
		return int(x), nil
	case uint64:
		return int(x), nil
	case float32:
		return int(x), nil
	case float64:
		return int(x), nil
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return 0, fmt.Errorf("cannot parse int from %q", x)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

func toUint64(v any) (uint64, error) {
	switch x := v.(type) {
	case uint64:
		return x, nil
	case uint:
		return uint64(x), nil
	case uint8:
		return uint64(x), nil
	case uint16:
		return uint64(x), nil
	case uint32:
		return uint64(x), nil
	case int:
		if x < 0 {
			return 0, fmt.Errorf("negative value %d", x)
		}
		return uint64(x), nil
	case int8:
		if x < 0 {
			return 0, fmt.Errorf("negative value %d", x)
		}
		return uint64(x), nil
	case int16:
		if x < 0 {
			return 0, fmt.Errorf("negative value %d", x)
		}
		return uint64(x), nil
	case int32:
		if x < 0 {
			return 0, fmt.Errorf("negative value %d", x)
		}
		return uint64(x), nil
	case int64:
		if x < 0 {
			return 0, fmt.Errorf("negative value %d", x)
		}
		return uint64(x), nil
	case float32:
		if x < 0 {
			return 0, fmt.Errorf("negative value %v", x)
		}
		return uint64(x), nil
	case float64:
		if x < 0 {
			return 0, fmt.Errorf("negative value %v", x)
		}
		return uint64(x), nil
	case string:
		n, err := strconv.ParseUint(strings.TrimSpace(x), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse uint64 from %q", x)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to uint64", v)
	}
}

func toFloat64(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case float32:
		return float64(x), nil
	case int:
		return float64(x), nil
	case int8:
		return float64(x), nil
	case int16:
		return float64(x), nil
	case int32:
		return float64(x), nil
	case int64:
		return float64(x), nil
	case uint:
		return float64(x), nil
	case uint8:
		return float64(x), nil
	case uint16:
		return float64(x), nil
	case uint32:
		return float64(x), nil
	case uint64:
		return float64(x), nil
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse float64 from %q", x)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

func toBool(v any) (bool, error) {
	switch x := v.(type) {
	case bool:
		return x, nil
	case string:
		b, err := strconv.ParseBool(strings.TrimSpace(x))
		if err != nil {
			return false, fmt.Errorf("cannot parse bool from %q", x)
		}
		return b, nil
	case int:
		return x != 0, nil
	case int8:
		return x != 0, nil
	case int16:
		return x != 0, nil
	case int32:
		return x != 0, nil
	case int64:
		return x != 0, nil
	case uint:
		return x != 0, nil
	case uint8:
		return x != 0, nil
	case uint16:
		return x != 0, nil
	case uint32:
		return x != 0, nil
	case uint64:
		return x != 0, nil
	case float32:
		return x != 0, nil
	case float64:
		return x != 0, nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", v)
	}
}
