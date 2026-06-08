package domain

type SponsorBlockSegment struct {
	UUID          string
	Category      string
	ActionType    string
	StartTime     float64
	EndTime       float64
	VideoDuration float64
	Locked        int
	Votes         int
	Description   string
}

func NewSponsorBlockSegment(
	uuid string,
	category string,
	actionType string,
	startTime float64,
	endTime float64,
	videoDuration float64,
	locked int,
	votes int,
	description string,
) SponsorBlockSegment {
	return SponsorBlockSegment{
		UUID:          uuid,
		Category:      category,
		ActionType:    actionType,
		StartTime:     startTime,
		EndTime:       endTime,
		VideoDuration: videoDuration,
		Locked:        locked,
		Votes:         votes,
		Description:   description,
	}
}
