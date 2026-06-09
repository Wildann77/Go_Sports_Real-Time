package enums

type MatchStatus string

const (
	StatusScheduled MatchStatus = "scheduled"
	StatusLive      MatchStatus = "live"
	StatusFinished  MatchStatus = "finished"
)

func (s MatchStatus) IsValid() bool {
	switch s {
	case StatusScheduled, StatusLive, StatusFinished:
		return true
	}
	return false
}
