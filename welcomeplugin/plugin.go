package welcomeplugin

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/iopred/bruxism"
)

const welcomeChannelName = "introductions"

type WelcomePlugin struct {
	discord      *bruxism.Discord
	ownerUserID  string
	MessageStats map[string]*stats
}

func New(d *bruxism.Discord, ownerUserID string) bruxism.Plugin {
	return &WelcomePlugin{
		discord:      d,
		ownerUserID:  ownerUserID,
		MessageStats: map[string]*stats{},
	}
}

func (w *WelcomePlugin) Name() string {
	return "welcome"
}

func (w *WelcomePlugin) Load(bot *bruxism.Bot, service bruxism.Service, data []byte) error {
	if service.Name() != bruxism.DiscordServiceName {
		panic("Welcome Plugin only supports Discord.")
	}

	if data != nil {
		if err := json.Unmarshal(data, w); err != nil {
			log.Println("welcomeplugin: loading data err:", err)
		}
	}

	go w.setupListeners()
	return nil
}

func (w *WelcomePlugin) setupListeners() {
	w.discord.Session.State.TrackPresences = true
	w.discord.Session.AddHandler(w.guildMemberAddHandler)
}

func (w *WelcomePlugin) guildMemberAddHandler(s *discordgo.Session, evt *discordgo.GuildMemberAdd) {
	w.sendNewMemberMessage(s, evt.Member)
}
func (w *WelcomePlugin) sendNewMemberMessage(s *discordgo.Session, evt *discordgo.Member) {
	g, err := s.Guild(evt.GuildID)
	if err != nil {
		log.Println("unable to fetch guild id", err)
		return
	}

	onlineMems := 0
	realMems := 0

	realRoleID := ""
	allRoles, err := w.discord.Session.GuildRoles(g.ID)
	if err != nil {
		log.Println("unable to fetch roles")
		allRoles = []*discordgo.Role{}
	}
	for _, r := range allRoles {
		if r.Name == "Real" {
			realRoleID = r.ID
		}
	}

	for _, m := range g.Members {
		if containsRole(realRoleID, m.Roles) {
			realMems++
		}

		p, _ := s.State.Presence(g.ID, m.User.ID)
		if p != nil && p.Status != discordgo.StatusOffline {
			onlineMems++
		}
	}

	intoChannel := "#introductions"

	for _, c := range g.Channels {
		if c.Name == welcomeChannelName {
			intoChannel = c.Mention()
			break
		}
	}
	gstats := w.guildStats(evt.GuildID)

	msg := renderMessage(messageVars{
		ServerName:          g.Name,
		User:                evt.User.Mention(),
		TotalUsersCount:     g.MemberCount,
		OnlineUsersCount:    onlineMems,
		RealUsersCount:      realMems,
		MessagesToday:       gstats.today(),
		MessagesLastWeek:    gstats.week(),
		IntroductionChannel: intoChannel,
	})

	ch, err := s.UserChannelCreate(evt.User.ID)
	if err != nil {
		log.Printf("unable create user channel to %s: %+v", evt.Nick, err)
		return
	}

	s.ChannelMessageSend(ch.ID, msg)
}

func containsRole(roleID string, roles []string) bool {
	for _, r := range roles {
		if roleID == r {
			return true
		}
	}
	return false
}

func (w *WelcomePlugin) Help(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message, detailed bool) []string {
	return []string{
		"welcomes people to the chat",
		bruxism.CommandHelp(service, "welcome-me", "", "To receive the welcome message as a dm.")[0],
		bruxism.CommandHelp(service, "msgcount", "", "To ask the bot to send the current message stats.")[0],
	}
}

func (w *WelcomePlugin) Message(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message) {
	if message.Type() == bruxism.MessageTypeCreate {
		w.guildStats(w.guildID(message)).increment(time.Now())
	}

	if strings.HasPrefix(message.Message(), "!debug") && message.UserID() == w.ownerUserID {
		w.guildStats(w.guildID(message)).printBuckets()
	}

	if bruxism.MatchesCommand(service, "msgcount", message) {
		response := fmt.Sprintf("```%s```", w.guildStats(w.guildID(message)).String())
		service.SendMessage(message.Channel(), response)
	}

	if bruxism.MatchesCommand(service, "welcome-me", message) {
		guildID := w.guildID(message)

		guildMember, err := w.discord.Session.GuildMember(guildID, message.UserID())
		guildMember.GuildID = guildID
		if err != nil {
			log.Println("unable to get guild member from channel id")
			return
		}
		w.sendNewMemberMessage(w.discord.Session, guildMember)
	}
}

func (w *WelcomePlugin) guildID(msg bruxism.Message) string {
	ch, err := w.discord.Session.Channel(msg.Channel())
	if err != nil {
		log.Println("unable to get guildID", err)
		return ""
	}
	return ch.GuildID
}

func (w *WelcomePlugin) guildStats(guildID string) *stats {
	if s, ok := w.MessageStats[guildID]; ok {
		return s
	}

	s := newStats(10)
	w.MessageStats[guildID] = s
	return s
}

func (w *WelcomePlugin) Save() ([]byte, error) {
	return json.Marshal(w)
}

func (w *WelcomePlugin) Stats(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message) []string {
	return nil
}
