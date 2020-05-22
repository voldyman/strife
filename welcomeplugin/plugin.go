package welcomeplugin

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/iopred/bruxism"
)

type WelcomePlugin struct {
	discord      *bruxism.Discord
	ownerUserID  string
	MessageStats *stats
}

func New(d *bruxism.Discord, ownerUserID string) bruxism.Plugin {
	return &WelcomePlugin{
		discord:      d,
		ownerUserID:  ownerUserID,
		MessageStats: newStats(10),
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

	go w.setupJoinListener()

	return nil
}

func (w *WelcomePlugin) setupJoinListener() {
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
		if p != nil && (p.Status == discordgo.StatusIdle || p.Status == discordgo.StatusOnline) {
			onlineMems++
		}
	}

	msg := renderMessage(messageVars{
		ServerName:       g.Name,
		User:             evt.User.Mention(),
		TotalUsersCount:  g.MemberCount,
		OnlineUsersCount: onlineMems,
		RealUsersCount:   realMems,
		MessagesToday:    w.MessageStats.today(),
		MessagesLastWeek: w.MessageStats.week(),
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
	}
}

func (w *WelcomePlugin) Message(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message) {
	if message.Type() == bruxism.MessageTypeCreate {
		w.MessageStats.increment(time.Now())
	}

	if strings.HasPrefix(message.Message(), "!print") && message.UserID() == w.ownerUserID {
		ch, err := w.discord.Session.Channel(message.Channel())
		if err != nil {
			log.Println("unable to get channel from channel id")
			return
		}

		guildMember, err := w.discord.Session.GuildMember(ch.GuildID, message.UserID())
		guildMember.GuildID = ch.GuildID
		if err != nil {
			log.Println("unable to get guild member from channel id")
			return
		}
		w.sendNewMemberMessage(w.discord.Session, guildMember)
	}
}

func (w *WelcomePlugin) Save() ([]byte, error) {
	return json.Marshal(w)
}

func (w *WelcomePlugin) Stats(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message) []string {
	return nil
}
