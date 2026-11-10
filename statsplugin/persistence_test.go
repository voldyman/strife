package statsplugin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/voldyman/bitstats"
)

func newTestPlugin() *StatsPlugin {
	return &StatsPlugin{
		MessageStats: map[string]*StatsRecorder{},
		GuildStats:   map[string]*bitstats.Stats{},
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	p := newTestPlugin()
	s := bitstats.New()
	s.Add("2022-01-01", "message:10", 42)
	p.GuildStats["g1"] = s

	data, err := p.Save()
	assert.NoError(t, err)

	loaded := newTestPlugin()
	assert.NoError(t, loaded.loadState(data))
	vals, ok := loaded.GuildStats["g1"].Values("2022-01-01", "message:10")
	assert.True(t, ok)
	assert.Equal(t, []uint64{42}, vals)
}

// state written before the binary format: GuildStats values are inline JSON objects
func TestLoadLegacyState(t *testing.T) {
	legacy := []byte(`{"MessageStats":{},"GuildStats":{"g1":{"Partitions":{"2022-01-01":[{"Name":"message:10","Value":[42,43]}]}}}}`)

	p := newTestPlugin()
	assert.NoError(t, p.loadState(legacy))
	vals, ok := p.GuildStats["g1"].Values("2022-01-01", "message:10")
	assert.True(t, ok)
	assert.Equal(t, []uint64{42, 43}, vals)

	// and re-saving migrates it to the new format
	data, err := p.Save()
	assert.NoError(t, err)
	var raw struct{ GuildStats map[string]json.RawMessage }
	assert.NoError(t, json.Unmarshal(data, &raw))
	assert.Equal(t, byte('"'), raw.GuildStats["g1"][0], "guild stats should be saved as a binary blob")
}

func TestLoadSkipsCorruptGuild(t *testing.T) {
	data := []byte(`{"GuildStats":{"bad":"AAAA","good":{"Partitions":{"d":[{"Name":"e","Value":[1]}]}}}}`)
	p := newTestPlugin()
	assert.NoError(t, p.loadState(data))
	assert.NotContains(t, p.GuildStats, "bad")
	assert.Contains(t, p.GuildStats, "good")
}

func TestLoadInvalidJSONKeepsState(t *testing.T) {
	p := newTestPlugin()
	p.GuildStats["keep"] = bitstats.New()
	assert.Error(t, p.loadState([]byte("not json")))
	assert.Contains(t, p.GuildStats, "keep")
}
