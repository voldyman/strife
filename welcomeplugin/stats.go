package welcomeplugin

import (
	"container/ring"
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
	Buckets *ring.Ring
}

func newStats(days int) *stats {
	return &stats{
		Buckets: ring.New(days),
	}
}

func (s *stats) increment(t time.Time) {
	if s.Buckets.Value == nil {
		s.Buckets.Value = newBucket(t)
	}
	for !s.curBucket().Add(t, 1) {
		s.moveBucketForward()
	}
}

func (s *stats) curBucket() *bucket {
	if s.Buckets == nil || s.Buckets.Value == nil { // first time
		b := newBucket(time.Now())
		s.Buckets.Value = b
		return b
	}

	return s.Buckets.Value.(*bucket)
}

func (s *stats) moveBucketForward() *bucket {
	curRing := s.curBucket()
	newBucket := newBucket(curRing.End.Add(1 * time.Hour))

	nextRing := s.Buckets.Next()
	nextRing.Value = newBucket
	s.Buckets = nextRing
	return newBucket
}

func (s *stats) today() int {
	return s.curBucket().Count
}

func (s *stats) yesterday() int {
	if s.Buckets == nil || s.Buckets.Prev() == nil {
		return 0
	}
	b := s.Buckets.Prev().Value.(*bucket)
	return b.Count
}

func (s *stats) week() int {
	if s.Buckets == nil {
		return 0
	}
	weekDate := time.Now().Add(-7 * 24 * time.Hour)
	count := 0
	s.Buckets.Do(func(v interface{}) {
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
	bucketCount := 0
	s.Buckets.Do(func(v interface{}) {
		defer func() { bucketCount++ }()
		if v == nil {
			fmt.Println(bucketCount, "<nil>")
			return
		}
		b := v.(*bucket)
		fmt.Println(bucketCount, b.Count, b.End)
	})
}
