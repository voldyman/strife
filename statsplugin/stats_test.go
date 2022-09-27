package statsplugin

import (
	"encoding/json"
	"io/fs"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const durationDay = 24 * time.Hour

var testClock = localClock(timeZone)

func TestStatsToday(t *testing.T) {
	s := NewStatsRecorder(testClock, 3)
	s.Increment(testClock.Now())

	if s.Today() != 1 {
		t.Fatal("increment wasn't recorded")
	}

	if s.Week() != 1 {
		t.Fatal("week count for one day was incorrect")
	}
}

func TestRealisticAdd(t *testing.T) {
	s := NewStatsRecorder(testClock, 10)

	distribution := []int{1, 4, 3, 2, 1, 3, 9, 5, 6, 8}
	for day := len(distribution) - 1; day >= 0; day-- {
		count := distribution[day]

		when := testClock.Now().Add(-(time.Duration(day) * durationDay))
		for i := 0; i < count; i++ {
			s.Increment(when)
		}
	}

	if s.Week() != 28 {
		s.printBuckets()
		t.Fatal("week count was incorrect, expected: 23 but was:", s.Week())
	}

	if s.Today() != 1 {
		t.Fatal("today count was incorrect, expected: 1 but was:", s.Today())
	}

	if s.Yesterday() != 4 {
		t.Fatal("yesterday count was incorrect, expected: 4 but was:", s.Yesterday())
	}
}

func TestMarshaling(t *testing.T) {
	s := NewStatsRecorder(testClock, 10)
	distribution := []int{1, 4, 3, 2, 1, 3, 9, 5, 6, 8}
	for day := len(distribution) - 1; day >= 0; day-- {
		count := distribution[day]

		when := testClock.Now().Add(-(time.Duration(day) * durationDay))
		for i := 0; i < count; i++ {
			s.Increment(when)
		}
	}
	ser, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("unable to marshal stats recorder: %+v", err)
	}
	newStatsRecorder := NewStatsRecorder(testClock, 10)
	err = json.Unmarshal(ser, newStatsRecorder)
	if err != nil {
		t.Fatalf("unable to unmarshall stats recorder: %+v", err)
	}
	if s.Week() != newStatsRecorder.Week() {
		newStatsRecorder.printBuckets()
		t.Fatalf("week count was incorrect, expected: %d but was: %d", s.Week(), newStatsRecorder.Week())
	}

	if s.Today() != newStatsRecorder.Today() {
		t.Fatalf("today count was incorrect, expected: %d but was: %d", s.Today(), newStatsRecorder.Today())
	}

	if s.Yesterday() != newStatsRecorder.Yesterday() {
		t.Fatalf("yesterday count was incorrect, expected: %d but was: %d", s.Yesterday(), newStatsRecorder.Yesterday())
	}
}

func TestNewBucket(t *testing.T) {
	initDate := time.Date(2022, time.April, 2, 17, 20, 0, 0, time.Local)
	bucket := newBucket(initDate)
	// the bucket should expire at the last hour of the day
	assert.Equal(t, bucket.End.Day(), 2)
	assert.Equal(t, bucket.End.Hour(), 23)
}

func TestHourlyCounters(t *testing.T) {
	// testgen.py
	distribution := [][]int{
		{8, 4, 1, 4, 3, 5, 4, 1, 4, 2, 4, 8, 3, 5, 4, 3, 9, 3, 9, 3, 2, 3, 2, 8}, // 102
		{6, 6, 1, 0, 2, 2, 4, 4, 0, 3, 6, 6, 3, 7, 5, 8, 6, 6, 4, 7, 8, 1, 8, 6}, // 109
		{9, 3, 4, 7, 6, 8, 4, 4, 7, 6, 5, 8, 8, 4, 0, 2, 2, 3, 1, 4, 1, 4, 4, 9}, // 113
		{9, 1, 3, 9, 9, 6, 0, 0, 7, 6, 9, 0, 6, 5, 8, 0, 1, 2, 4, 5, 3, 1, 9, 2}, // 105
		{8, 6, 5, 1, 5, 2, 6, 4, 6, 7, 7, 1, 9, 5, 3, 2, 5, 2, 3, 0, 7, 2, 4, 6}, // 106
		{8, 0, 4, 9, 3, 2, 2, 8, 8, 7, 4, 8, 4, 7, 9, 8, 8, 2, 4, 6, 3, 3, 0, 2}, // 119
		{9, 5, 9, 6, 1, 9, 0, 8, 5, 2, 7, 0, 0, 6, 3, 3, 5, 9, 5, 7, 5, 2, 6, 2}, // 114
	}

	testDays := len(distribution)
	s := NewStatsRecorder(testClock, testDays)
	// start today - len(tests) days ago
	start := testClock.Now().In(timeZone).Add(-(time.Duration(testDays) * 24 * time.Hour))
	// align start time to 0th hour of the test start day
	start = start.Add(-time.Duration(start.Hour()) * time.Hour)
	for day, hourlyCounts := range distribution {
		// create day for Nth day
		dayTime := start.Add(time.Duration(day) * 24 * time.Hour)
		for hr, counts := range hourlyCounts {
			// start from 0th hour till 23
			when := dayTime.Add(time.Hour * time.Duration(hr))
			for i := 0; i < counts; i++ {
				s.Increment(when)
			}
		}
	}
	s.printBuckets()

	matrixResult := s.WeekMatrix()
	weekMatrix := matrixResult.matrix

	totalCount := 0
	matrixCount := 0

	assert.Equal(t, len(distribution), len(weekMatrix), "Generated test days should be same as input distribution")
	for i := range distribution {
		for j := range distribution[i] {
			totalCount += distribution[i][j]
			matrixCount += weekMatrix[i][j]
		}
	}
	assert.Equal(t, totalCount, matrixCount)

	assert.Equal(t, sumDay(weekMatrix[0]), 102, "day 0 does not match expected total")
	assert.Equal(t, sumDay(weekMatrix[1]), 109, "day 1 does not match expected total")
	assert.Equal(t, sumDay(weekMatrix[2]), 113, "day 2 does not match expected total")
	assert.Equal(t, sumDay(weekMatrix[3]), 105, "day 3 does not match expected total")
	assert.Equal(t, sumDay(weekMatrix[4]), 106, "day 4 does not match expected total")
	assert.Equal(t, sumDay(weekMatrix[5]), 119, "day 5 does not match expected total")
	assert.Equal(t, sumDay(weekMatrix[6]), 114, "day 6 does not match expected total")
	w, _ := matrixResult.Plot()
	data, _ := ioutil.ReadAll(w)
	ioutil.WriteFile("image.png", data, fs.ModePerm)
}

func sumDay(counts [24]int) int {
	total := 0
	for _, c := range counts {
		total += c
	}
	return total
}

func TestUnmarshingStats(t *testing.T) {
	storedJSON := `{"MessageStats":{"680975393188347993":{"Days":10,"Buckets":[{"Count":1068,"Hourly":[0,2,0,0,4,5,0,0,9,0,0,0,1,0,0,0,0,0,0,0,0,0,0,0],"End":"2022-09-26T23:21:22.220437159-07:00"}]},"707620933841453186":{"Days":10,"Buckets":[{"Count":2,"Hourly":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,2,0,0,0,0],"End":"2022-09-26T23:57:26.599272-07:00"}]},"816206276862148618":{"Days":10,"Buckets":[{"Count":26,"Hourly":[0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0],"End":"2022-09-26T23:26:04.726608499-07:00"}]}}}`

	statsPlugin := &StatsPlugin{}
	err := json.Unmarshal([]byte(storedJSON), statsPlugin)
	assert.Nil(t, err)
	w, _ := statsPlugin.MessageStats["680975393188347993"].WeekMatrix().Plot()
	data, _ := ioutil.ReadAll(w)
	ioutil.WriteFile("image.png", data, fs.ModePerm)
	t.Fail()
}
