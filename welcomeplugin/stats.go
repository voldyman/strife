package welcomeplugin

import (
	"container/ring"
	"encoding/json"
	"fmt"
	"time"
)

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
		s.buckets.Value = newBucket(t)
	}
	for !s.curBucket().Add(t, 1) {
		s.moveBucketForward()
	}
}

func (s *stats) curBucket() *bucket {
	if s.buckets == nil || s.buckets.Value == nil { // first time
		b := newBucket(time.Now())
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
	weekDate := time.Now().Add(-7 * 24 * time.Hour)
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
	if s.buckets == nil {
		fmt.Println("nil buckets")
		return
	}
	bucketCount := 0
	s.buckets.Do(func(v interface{}) {
		defer func() { bucketCount++ }()
		if v == nil {
			fmt.Println(bucketCount, "<nil>")
			return
		}
		b := v.(*bucket)
		fmt.Println(bucketCount, b.Count, b.End)
	})
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
	for _, bs := range spec.Buckets {
		s.buckets = s.buckets.Next()
		s.buckets.Value = &bucket{
			Count: bs.Count,
			End:   bs.End,
		}
	}
	return nil
}
