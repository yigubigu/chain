package bc

type TimeRange struct {
	body struct {
		MinTimeMS, MaxTimeMS uint64
		ExtHash              extHash
	}
}

const typeTimeRange = "timerange1"

func (TimeRange) Type() string             { return typeTimeRange }
func (tr *TimeRange) Body() interface{}    { return &tr.body }
func (tr *TimeRange) Witness() interface{} { return nil }

func newTimeRange(minTimeMS, maxTimeMS uint64) *TimeRange {
	tr := new(TimeRange)
	tr.body.MinTimeMS = minTimeMS
	tr.body.MaxTimeMS = maxTimeMS
	return tr
}
