package main

import (
	"strconv"
)

func toString(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		if x == "" {
			return "", false
		}
		return x, true
	case float64:
		return strconv.Itoa(int(x)), true
	case int:
		return strconv.Itoa(x), true
	case int64:
		return strconv.FormatInt(x, 10), true
	default:
		return "", false
	}
}
func boolFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}
