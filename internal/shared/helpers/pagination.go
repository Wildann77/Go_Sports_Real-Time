package helpers

import (
	"sports-dashboard/internal/shared/constants"
	"strconv"
)

func ParseLimit(limitStr string, defaultLimit int) int {
	if limitStr == "" {
		return defaultLimit
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return defaultLimit
	}
	if limit > constants.MaxLimit {
		return constants.MaxLimit
	}
	return limit
}
