package statsplugin

import (
	"bytes"
	"container/ring"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/palette/brewer"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

var timeZone *time.Location = loadLocation("America/Vancouver") // where this bot works

type Clock interface {
	Now() time.Time
}

type LocalClock struct {
	zone *time.Location
}

func localClock(zone *time.Location) Clock {
	return &LocalClock{
		zone: zone,
	}
}
func (c *LocalClock) Now() time.Time {
	return time.Now().In(c.zone)
}

func loadLocation(loc string) *time.Location {
	timeZone, err := time.LoadLocation(loc)
	if err != nil {
		panic(err)
	}
	return timeZone
}

type StatsRecorder struct {
	dayBuckets *ring.Ring
	clock      Clock
	Days       int
}

func NewStatsRecorder(clock Clock, days int) *StatsRecorder {
	return &StatsRecorder{
		Days:       days,
		dayBuckets: ring.New(days),
		clock:      clock,
	}
}

func (s *StatsRecorder) Increment(t time.Time) {
	if s.dayBuckets == nil {
		s.dayBuckets = ring.New(s.Days)
	}
	if s.dayBuckets.Value == nil {
		s.dayBuckets.Value = newBucket(t)
	}
	for !s.curBucket().Add(t, 1) {
		s.moveBucketForward()
	}
}

func (s *StatsRecorder) Today() int {
	if s.dayBuckets == nil {
		return 0
	}
	return s.curBucket().Count
}

func (s *StatsRecorder) Yesterday() int {
	if s.dayBuckets == nil || s.dayBuckets.Prev() == nil {
		return 0
	}
	b := s.dayBuckets.Prev().Value.(*dayBucket)
	return b.Count
}

func (s *StatsRecorder) Week() int {
	if s.dayBuckets == nil {
		return 0
	}
	count := 0
	weekDate := s.clock.Now().Add(-7 * 24 * time.Hour)
	s.dayBuckets.Do(func(v interface{}) {
		if v == nil {
			return
		}
		b, ok := v.(*dayBucket)
		if !ok {
			return
		}

		if b.End.After(weekDate) {
			count += b.Count
		}
	})

	return count
}
func (s *StatsRecorder) WeekMatrix() *WeekMsgCountMatrix {
	weekDate := s.clock.Now().Add(-7 * 24 * time.Hour)
	result := &WeekMsgCountMatrix{
		matrix:    [7][24]int{},
		startDate: weekDate,
	}
	if s.dayBuckets == nil {
		return result
	}

	s.dayBuckets.Do(func(v interface{}) {
		if v == nil {
			return
		}
		b, ok := v.(*dayBucket)
		if !ok {
			return
		}

		dayDiff := b.End.Day() - weekDate.Day() - 1
		if dayDiff < 0 || dayDiff >= 7 {
			log.Printf("date diff %d < 0", dayDiff)
			return
		}

		result.matrix[dayDiff] = b.Hourly
	})
	return result
}

func (s *StatsRecorder) curBucket() *dayBucket {
	if s.dayBuckets == nil || s.dayBuckets.Value == nil { // first time
		b := newBucket(s.clock.Now())
		s.dayBuckets.Value = b
		return b
	}

	return s.dayBuckets.Value.(*dayBucket)
}

func (s *StatsRecorder) moveBucketForward() *dayBucket {
	curRing := s.curBucket()
	newBucket := newBucket(curRing.End.Add(6 * time.Hour))

	nextRing := s.dayBuckets.Next()
	nextRing.Value = newBucket
	s.dayBuckets = nextRing
	return newBucket
}

func (s *StatsRecorder) printBuckets() {
	fmt.Println(s.String())
}

func (s *StatsRecorder) String() string {
	if s.dayBuckets == nil {
		return "nil buckets"
	}
	var out strings.Builder
	bucketCount := 0
	iter := s.dayBuckets.Next()
	iter.Do(func(v interface{}) {
		defer func() { bucketCount++ }()
		if v == nil {
			out.WriteString(fmt.Sprintf("%d: %s   -\n", bucketCount, "<nil>"))

		}
		if b, ok := v.(*dayBucket); ok {
			out.WriteString(fmt.Sprintf("%d: %5d %s\n", bucketCount, b.Count, b.End.Format(time.RFC1123)))
		}
	})

	return out.String()
}

type bucketSpec struct {
	Count  int
	Hourly [24]int
	End    time.Time
}

type statsSpec struct {
	Days    int
	Buckets []bucketSpec
}

func (s StatsRecorder) MarshalJSON() ([]byte, error) {
	statsSpec := statsSpec{
		Days:    s.Days,
		Buckets: []bucketSpec{},
	}

	s.dayBuckets.Do(func(v interface{}) {
		if v == nil {
			return
		}
		b := v.(*dayBucket)
		statsSpec.Buckets = append(statsSpec.Buckets, bucketSpec{
			Count:  b.Count,
			End:    b.End,
			Hourly: b.Hourly,
		})
	})
	return json.Marshal(statsSpec)
}

func (s *StatsRecorder) UnmarshalJSON(b []byte) error {
	var spec statsSpec
	if err := json.Unmarshal(b, &spec); err != nil {
		return err
	}
	s.Days = spec.Days
	s.dayBuckets = ring.New(s.Days)
	s.clock = localClock(timeZone)

	sort.Slice(spec.Buckets, func(i, j int) bool { return spec.Buckets[i].End.Before(spec.Buckets[j].End) })

	for _, bs := range spec.Buckets {
		s.dayBuckets = s.dayBuckets.Next()
		s.dayBuckets.Value = &dayBucket{
			Count:  bs.Count,
			End:    bs.End.In(timeZone),
			Hourly: bs.Hourly,
		}
	}

	return nil
}

type dayBucket struct {
	End    time.Time
	Count  int
	Hourly [24]int
}

func newBucket(forDay time.Time) *dayBucket {
	ends := forDay.Add(time.Duration(24-forDay.Hour()-1) * time.Hour)
	return &dayBucket{
		End:    ends,
		Count:  0,
		Hourly: [24]int{},
	}
}

func (b *dayBucket) Add(t time.Time, c int) bool {
	if t.After(b.End) {
		return false
	}

	b.Hourly[t.Hour()] += c
	b.Count += c
	return true
}

type WeekMsgCountMatrix struct {
	matrix    [7][24]int
	startDate time.Time
}

func (m *WeekMsgCountMatrix) Dims() (c, r int) {
	return 7, 24
}
func (m *WeekMsgCountMatrix) X(c int) float64 {
	return float64(c)
}
func (m *WeekMsgCountMatrix) Y(r int) float64 {
	return float64(r)
}
func (m *WeekMsgCountMatrix) Z(c, r int) float64 {
	return float64(m.matrix[c][r])
}

func (m *WeekMsgCountMatrix) Plot() (io.Reader, error) {
	total := 7 * 24
	max := -1
	for _, day := range m.matrix {
		for _, hr := range day {
			if max < hr {
				max = hr
			}
		}
	}

	days := make([]string, 7)
	timeIter := m.startDate
	for i := range days {
		days[i] = timeIter.Format("2-Jan")
		timeIter = timeIter.Add(24 * time.Hour)
	}
	hours := make([]string, 24)
	for i := range hours {
		hr := i + 1
		suffix := "am"

		switch hr {
		case 12:
			suffix = "pm"
		case 24:
			hr = 12
			suffix = "am"
		default:
			if hr > 12 {
				hr = hr - 12
				suffix = "pm"
			}
		}

		hours[i] = fmt.Sprintf("%d%s", hr, suffix)
	}
	paletteSize := total * max
	if paletteSize == 0 {
		paletteSize = 100
	}
	colorpalette, err := brewer.GetPalette(brewer.TypeSequential, "YlGnBu", 9)
	if err != nil {
		panic(err)
	}
	hm := plotter.NewHeatMap(m, colorpalette)
	pt := plot.New()
	pt.Title.Text = "Weekly Activity"
	// pt.X.Label.Text = "Days"
	// pt.Y.Label.Text = "Hours"
	pt.X.Tick.Marker = ticks(days)
	pt.Y.Tick.Marker = ticks(hours)
	pt.Add(hm)
	wt, err := pt.WriterTo(5*vg.Inch, 5*vg.Inch, "png")
	if err != nil {
		return nil, errors.Wrap(err, "unable to create plot writer")
	}
	buf := bytes.Buffer{}
	_, err = wt.WriteTo(&buf)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to writer plot to buffer")
	}
	return &buf, nil
}

// internal data structure for heatmap ticks
type ticks []string

func (t ticks) Ticks(min, max float64) []plot.Tick {
	var retVal []plot.Tick
	for i := math.Trunc(min); i <= max; i++ {
		label := ""
		if int(i) < len(t) {
			label = t[int(i)]
		}

		retVal = append(retVal, plot.Tick{Value: i, Label: label})
	}
	return retVal
}
