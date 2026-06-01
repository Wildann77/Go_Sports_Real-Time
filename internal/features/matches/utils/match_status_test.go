package utils

import (
	"sports-dashboard/internal/shared/enums"
	"testing"
	"time"
)

func TestGetMatchStatus(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		startTime time.Time
		endTime   time.Time
		expected  string
	}{
		{
			name:      "Scheduled - Future Start Time",
			startTime: now.Add(2 * time.Hour),
			endTime:   now.Add(4 * time.Hour),
			expected:  string(enums.StatusScheduled),
		},
		{
			name:      "Finished - Past End Time",
			startTime: now.Add(-4 * time.Hour),
			endTime:   now.Add(-2 * time.Hour),
			expected:  string(enums.StatusFinished),
		},
		{
			name:      "Live - Currently Ongoing",
			startTime: now.Add(-1 * time.Hour),
			endTime:   now.Add(1 * time.Hour),
			expected:  string(enums.StatusLive),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := GetMatchStatus(tt.startTime, tt.endTime)
			if status != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, status)
			}
		})
	}
}
