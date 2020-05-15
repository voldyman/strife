package welcomeplugin

import (
	"time"
)

const durationDay = 24 * time.Hour

type stats struct {
	MaxRentention time.Duration
	MaxBuckets    int
	Buckets       []int
	LastTime      time.Time
}

func newStats(maxRentention time.Duration) *stats {
	maxBuckets := (maxRentention / durationDay) + 1

	return &stats{
		MaxRentention: maxRentention,
		MaxBuckets:    int(maxBuckets),
		Buckets:       make([]int, maxBuckets),
		LastTime:      time.Now(),
	}
}

func (s *stats) increment(t time.Time) {
	if len(s.Buckets) > s.MaxBuckets {
		s.Buckets = s.Buckets[0:s.MaxBuckets]
	}
	day := time.Now().Sub(t) / durationDay
	if day < 0 || day >= s.MaxRentention || int(day) >= len(s.Buckets) {
		return
	}

	bucket := s.LastTime.Sub(t) / durationDay
	s.LastTime = t
	if bucket > 0 {
		s.Buckets = append([]int{0}, s.Buckets...)
	}
	s.Buckets[day] = s.Buckets[day] + 1
}

func (s *stats) today() int {
	if len(s.Buckets) == 0 {
		return 0
	}
	return s.Buckets[0]
}

func (s *stats) yesterday() int {
	if len(s.Buckets) < 2 {
		return 0
	}
	return s.Buckets[1]
}

func (s *stats) week() int {
	count := 0
	for i := 0; i < len(s.Buckets) && i < 7; i++ {
		count = s.Buckets[i] + count
	}

	return count
}
