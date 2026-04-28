package validator

import "strings"

// Required reports whether all provided string values are non-empty after trimming.
func Required(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}
