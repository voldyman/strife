package statsplugin

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/iopred/bruxism"
	"github.com/pkg/errors"
	"github.com/voldyman/bitstats"
)

const bsHourlyFormat = "15"
const bsDateFormat = "2006-01-02"

func (w *StatsPlugin) recordMessage(guildID, channelID, userID string, typ bruxism.MessageType) {
	usrID, err := parseUserID(userID)
	if err != nil {
		log.Println(err)
		return
	}
	w.Lock()
	defer w.Unlock()
	stats, ok := w.GuildStats[guildID]
	if !ok {
		stats = bitstats.New()
		w.GuildStats[guildID] = stats
	}
	now := w.clock.Now()
	day := now.Format(bsDateFormat)
	hour := now.Format(bsHourlyFormat)
	stats.Add(day, string(typ)+":"+hour, usrID)

	// clean up extra days
	for stats.PartitionsCount() >= 10 {
		name, ok := stats.RemoveMinPartition()
		if ok {
			log.Println("Removed stats for day ", name)
		}
	}
}

func (w *StatsPlugin) statsToMatrix(guildID, eventPrefix string, fn func(*roaring64.Bitmap) int) (*WeekMsgCountMatrix, error) {
	matrix := [7][24]int{}
	stats, ok := w.GuildStats[guildID]
	now := w.clock.Now()
	if !ok {
		return nil, errors.Errorf("Guild Stats not found for %s", guildID)
	}
	for _, part := range stats.Partitions() {
		events, ok := stats.EventsByPrefix(part, eventPrefix)
		if !ok {
			break
		}
		partDay, err := time.Parse(bsDateFormat, part)
		if err != nil {
			log.Printf("Unable to parse partition name: %s, ignoring", part)
			continue
		}
		dayIndex := 7 - int(now.Sub(partDay).Hours()/24)
		if dayIndex < 0 || dayIndex >= 7 {
			log.Printf("Day index for %s is out of bounds [0, 6]: %d", part, dayIndex)
			continue
		}
		for _, event := range events {
			vals, ok := stats.ValuesSet(part, event)
			if !ok {
				log.Printf("Values Set not found for partition %s, event %s", part, event)
				continue
			}
			parts := strings.SplitN(event, ":", 2)
			if len(parts) < 2 {
				log.Printf("event %s doesn't have 2 parts: %+v", event, parts)
				continue
			}
			hourIndex, err := strconv.Atoi(parts[1])
			if err != nil {
				log.Printf("unable to parse %s into hour", parts[1])
				continue
			}
			matrix[dayIndex][hourIndex] = fn(vals)
		}
	}
	weekDate := w.clock.Now().Add(-6 * 24 * time.Hour)
	return &WeekMsgCountMatrix{
		matrix:    matrix,
		startDate: weekDate,
	}, nil
}

func parseUserID(userID string) (uint64, error) {
	usrID, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		return 0, errors.Errorf("unable to convert user id to uint64 \"%s\": %+v", userID, err)
	}
	return usrID, nil
}
