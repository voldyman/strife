package welcomeplugin

import (
	"container/ring"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

var timeZone *time.Location = loadLocation("America/Vancouver") // where this bot work

func loadLocation(loc string) *time.Location {
	timeZone, err := time.LoadLocation(loc)
	if err != nil {
		panic(err)
	}
	return timeZone
}

const durationDay = 24 * time.Hour

type bucket struct {
	End   time.Time
	Count int
}

func newBucket(forDay time.Time) *bucket {
	ends := forDay.Add(time.Duration(24-forDay.Hour()) * time.Hour)
	return &bucket{
		End:   ends,
		Count: 0,
	}
}

func (b *bucket) Add(t time.Time, c int) bool {
	if t.After(b.End) {
		return false
	}

	b.Count += c
	return true
}

type stats struct {
	buckets *ring.Ring
	Days    int
}

func newStats(days int) *stats {
	return &stats{
		Days:    days,
		buckets: ring.New(days),
	}
}

func (s *stats) increment(t time.Time) {
	if s.buckets == nil {
		s.buckets = ring.New(s.Days)
	}
	if s.buckets.Value == nil {
		s.buckets.Value = newBucket(t.In(timeZone))
	}
	for !s.curBucket().Add(t, 1) {
		s.moveBucketForward()
	}
}

func (s *stats) curBucket() *bucket {
	if s.buckets == nil || s.buckets.Value == nil { // first time
		b := newBucket(time.Now().In(timeZone))
		s.buckets.Value = b
		return b
	}

	return s.buckets.Value.(*bucket)
}

func (s *stats) moveBucketForward() *bucket {
	curRing := s.curBucket()
	newBucket := newBucket(curRing.End.Add(1 * time.Hour))

	nextRing := s.buckets.Next()
	nextRing.Value = newBucket
	s.buckets = nextRing
	return newBucket
}

func (s *stats) today() int {
	if s.buckets == nil {
		return 0
	}
	return s.curBucket().Count
}

func (s *stats) yesterday() int {
	if s.buckets == nil || s.buckets.Prev() == nil {
		return 0
	}
	b := s.buckets.Prev().Value.(*bucket)
	return b.Count
}

func (s *stats) week() int {
	if s.buckets == nil {
		return 0
	}
	weekDate := time.Now().In(timeZone).Add(-7 * 24 * time.Hour)
	count := 0
	s.buckets.Do(func(v interface{}) {
		if v == nil {
			return
		}
		b, ok := v.(*bucket)
		if !ok {
			return
		}

		if b.End.After(weekDate) {
			count += b.Count
		}
	})

	return count
}

func (s *stats) printBuckets() {
	fmt.Println(s.String())
}

func (s *stats) String() string {
	if s.buckets == nil {
		return "nil buckets"
	}
	var out strings.Builder
	bucketCount := 0
	iter := s.buckets.Next()
	iter.Do(func(v interface{}) {
		defer func() { bucketCount++ }()
		if v == nil {
			out.WriteString(fmt.Sprintf("%d: %s   -\n", bucketCount, "<nil>"))

		}
		if b, ok := v.(*bucket); ok {
			out.WriteString(fmt.Sprintf("%d: %5d %s\n", bucketCount, b.Count, b.End.Format(time.RFC1123)))
		}
	})

	return out.String()
}

type bucketSpec struct {
	Count int
	End   time.Time
}

type statsSpec struct {
	Days    int
	Buckets []bucketSpec
}

func (s stats) MarshalJSON() ([]byte, error) {
	statsSpec := statsSpec{
		Days:    s.Days,
		Buckets: []bucketSpec{},
	}

	s.buckets.Do(func(v interface{}) {
		if v == nil {
			return
		}
		b := v.(*bucket)
		statsSpec.Buckets = append(statsSpec.Buckets, bucketSpec{
			Count: b.Count,
			End:   b.End,
		})
	})
	return json.Marshal(statsSpec)
}

func (s *stats) UnmarshalJSON(b []byte) error {
	var spec statsSpec
	if err := json.Unmarshal(b, &spec); err != nil {
		return err
	}
	s.Days = spec.Days
	s.buckets = ring.New(s.Days)

	sort.Slice(spec.Buckets, func(i, j int) bool { return spec.Buckets[i].End.Before(spec.Buckets[j].End) })

	for _, bs := range spec.Buckets {
		s.buckets = s.buckets.Next()
		s.buckets.Value = &bucket{
			Count: bs.Count,
			End:   bs.End.In(timeZone),
		}
	}

	return nil
}
