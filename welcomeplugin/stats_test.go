package welcomeplugin

import (
	"testing"
	"time"
)

func TestStatsToday(t *testing.T) {
	s := newStats(3 * durationDay)
	s.increment(time.Now())

	if s.today() != 1 {
		t.Fatal("increment wasn't recorded")
	}

	if s.week() != 1 {
		t.Fatal("week count for one day was incorrect")
	}
}

func TestRealisticAdd(t *testing.T) {
	s := newStats(8 * durationDay)

	distribution := []int{1, 4, 3, 2, 1, 3, 9, 5, 6, 8}
	for day := len(distribution) - 1; day >= 0; day-- {
		count := distribution[day]

		when := time.Now().Add(-(time.Duration(day) * durationDay))
		for i := 0; i < count; i++ {
			s.increment(when)
		}
	}

	if s.week() != 23 {
		t.Fatal("week count was incorrect, expected: 23 but was:", s.week())
	}

	if s.today() != 1 {
		t.Fatal("today count was incorrect, expected: 1 but was:", s.today())
	}

	if s.yesterday() != 4 {
		t.Fatal("yesterday count was incorrect, expected: 4 but was:", s.yesterday())
	}
}
func TestStatsMultipleDaysForward(t *testing.T) {
	s := newStats(8 * durationDay)

	distribution := []int{1, 4, 3, 2, 1, 3, 9, 5, 6, 8}
	for day, count := range distribution {
		when := time.Now().Add(-(time.Duration(day) * durationDay))
		for i := 0; i < count; i++ {
			s.increment(when)
		}
	}

	if s.week() != 23 {
		t.Fatal("week count was incorrect, expected: 23 but was:", s.week())
	}

	if s.today() != 1 {
		t.Fatal("todayday count was incorrect, expected: 1 but was:", s.today())
	}

	if s.yesterday() != 4 {
		t.Fatal("yesterday count was incorrect, expected: 4 but was:", s.yesterday())
	}
}

func TestStatsMultipleDaysFuture(t *testing.T) {
	s := newStats(10 * durationDay)

	distribution := []int{1, 4, 3, 2, 1, 3, 9, 5, 6, 8}
	for day, count := range distribution {
		when := time.Now().Add(time.Duration(day) * durationDay)
		for i := 0; i < count; i++ {
			s.increment(when)
		}
	}

	// only first day is a valid increment, everything else is in the future
	if s.week() != 5 {
		t.Fatal("week count was incorrect, expected: 5 but was:", s.week())
	}

	if s.today() != 5 {
		t.Fatal("day count was incorrect, expected: 5 but was:", s.today())
	}

	if s.yesterday() != 0 {
		t.Fatal("yesterday count was incorrect, expected: 0 but was:", s.yesterday())
	}
}
