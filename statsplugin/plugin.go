package statsplugin

import (
	"encoding/json"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/iopred/bruxism"
)

type StatsPlugin struct {
	discord      *bruxism.Discord
	MessageStats map[string]*StatsRecorder
}

func New(d *bruxism.Discord) bruxism.Plugin {
	return &StatsPlugin{
		discord:      d,
		MessageStats: map[string]*StatsRecorder{},
	}
}

func (w *StatsPlugin) Name() string {
	return "stats"
}

func (w *StatsPlugin) Load(bot *bruxism.Bot, service bruxism.Service, data []byte) error {
	if service.Name() != bruxism.DiscordServiceName {
		panic("Welcome Plugin only supports Discord.")
	}

	if data != nil {
		if err := json.Unmarshal(data, w); err != nil {
			log.Println("StatsPlugin: loading data err:", err)
		}
	}

	go w.setupListeners()

	return nil
}

const applicationID = "meetup-post-application-id"

func (w *StatsPlugin) setupListeners() {
	w.discord.Session.State.TrackPresences = true
	for _, s := range w.discord.Sessions {
		for _, guild := range s.State.Guilds {
			w.discord.Session.ApplicationCommandCreate(applicationID, guild.ID, messageStatsCMD())
		}

	}
}

func messageStatsCMD() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		ID:          applicationID,
		Name:        "stats",
		Description: "Show server stats",
	}
}

func (w *StatsPlugin) Help(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message, detailed bool) []string {
	return []string{
		"message stats about the server",
		bruxism.CommandHelp(service, "stats", "", "To ask the bot to send the current message stats.")[0],
	}
}

func (w *StatsPlugin) Message(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message) {
	if message.Type() == bruxism.MessageTypeCreate {
		w.guildStats(w.guildID(message)).Increment(time.Now())
	}
}

func (w *StatsPlugin) guildID(msg bruxism.Message) string {
	ch, err := w.discord.Session.Channel(msg.Channel())
	if err != nil {
		log.Println("unable to get guildID", err)
		return ""
	}
	return ch.GuildID
}

func (w *StatsPlugin) guildStats(guildID string) *StatsRecorder {
	if s, ok := w.MessageStats[guildID]; ok {
		return s
	}

	s := NewStatsRecorder(localClock(timeZone), 10)
	w.MessageStats[guildID] = s
	return s
}

func (w *StatsPlugin) Save() ([]byte, error) {
	return json.Marshal(w)
}

func (w *StatsPlugin) Stats(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message) []string {
	return nil
}
