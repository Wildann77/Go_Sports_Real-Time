package utils

import (
	"sports-dashboard/internal/shared/enums"
	"time"
)

func GetMatchStatus(startTime, endTime time.Time) string {
	now := time.Now()
	if now.Before(startTime) {
		return string(enums.StatusScheduled)
	}
	if now.After(endTime) {
		return string(enums.StatusFinished)
	}
	return string(enums.StatusLive)
}
