package statsplugin

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/iopred/bruxism"
)

type StatsPlugin struct {
	discord      *bruxism.Discord
	MessageStats map[string]*StatsRecorder
	allowedRoles map[string][]string
}

func New(d *bruxism.Discord, allowedRoles map[string][]string) bruxism.Plugin {
	return &StatsPlugin{
		discord:      d,
		MessageStats: map[string]*StatsRecorder{},
		allowedRoles: allowedRoles,
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
			cmd, err := w.discord.Session.ApplicationCommandCreate(w.discord.Session.State.User.ID, guild.ID, messageStatsCMD())
			if err != nil {
				log.Print("unable to create command:", err)
				continue
			}
			log.Print("created stats command:", cmd.ApplicationID, "for guild:", guild.Name)
		}
		w.discord.Session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			if i.ApplicationCommandData().Name == "stats" {
				w.handleStatsCommand(s, i)
			}
		})
	}
}

func messageStatsCMD() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		ID:          applicationID,
		Name:        "stats",
		Description: "Show server stats",
	}
}

const protectedServer = "707620933841453186"

func (w *StatsPlugin) handleStatsCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID := ""
	if i.User != nil {
		userID = i.User.ID
	}
	if i.Member != nil {
		userID = i.Member.User.ID
	}
	log.Printf("responding to stats command from guild id: '%s' and user id: '%s'", i.GuildID, userID)
	if i.GuildID != protectedServer {
		w.sendStatsResponse(s, i)
		return
	}
	if w.isUserAllowed(i.GuildID, userID) {
		w.sendStatsResponse(s, i)
		return
	} else {
		guildName := fmt.Sprintf(i.GuildID)
		g, err := s.Guild(i.GuildID)
		if err == nil {
			guildName = g.Name
		}

		log.Printf("unauthorized user: '%s' requested stats on guild: '%s'", i.User.Username, guildName)
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Statistics is the discipline that concerns the collection, organization, analysis, interpretation, and presentation of data. In applying statistics to a scientific, industrial, or social problem, it is conventional to begin with a statistical population or a statistical model to be studied.",
			Embeds: []*discordgo.MessageEmbed{
				{Type: discordgo.EmbedTypeImage,
					Image: &discordgo.MessageEmbedImage{
						URL: "https://i.imgur.com/7Plfx7v.jpg",
					},
				},
			},
		},
	})
	if err != nil {
		log.Print("unable to respond")
	} else {
		log.Print("successfully responded to stats")
	}
}

func (w *StatsPlugin) isUserAllowed(guildID, userID string) bool {
	roles, ok := w.allowedRoles[guildID]
	if !ok {
		return true
	}

	guildRoles := fetchGuildRoles(w.discord.Session, guildID)
	allowedRoles := filterRoles(guildRoles, roles, guildID)

	guildMember, err := w.discord.Session.GuildMember(guildID, userID)
	if err != nil {
		log.Println("unable to get guild member for user:", userID, guildID, err)
		return false
	}

	for _, userRoleID := range guildMember.Roles {
		if _, ok := allowedRoles[userRoleID]; ok {
			return true
		}
	}
	return false
}

func (w *StatsPlugin) sendStatsResponse(s *discordgo.Session, i *discordgo.InteractionCreate) {
	stats, ok := w.MessageStats[i.GuildID]
	if !ok {
		w.respondWithError(s, i, "stats not found for guild")
		return
	}
	imgReader, err := stats.WeekMatrix().Plot()
	if err != nil {
		w.respondWithError(s, i, "unable to render activity plot: "+err.Error())
		return
	}
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Server Activity Stats",
			Files: []*discordgo.File{
				{Name: "activity_stats.png", ContentType: "image/png", Reader: imgReader},
			},
		},
	})
	if err != nil {
		log.Print("unable to respond")
	} else {
		log.Print("successfully responded to stats")
	}
}

func (w *StatsPlugin) respondWithError(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
		},
	})
	if err != nil {
		log.Print("unable to respond")
	} else {
		log.Print("successfully responded to stats")
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

func fetchGuildRoles(session *discordgo.Session, guildID string) map[string]*discordgo.Role {
	guildMap := map[string]*discordgo.Role{}

	guildRoles, err := session.GuildRoles(guildID)
	if err != nil {
		log.Println("unable to get guild roles for guild:", guildID, err)
		return guildMap
	}

	for _, role := range guildRoles {
		guildMap[role.Name] = role
	}

	return guildMap
}

func filterRoles(guildRoles map[string]*discordgo.Role, adminRoles []string, guildID string) map[string]*discordgo.Role {
	allowedRoles := map[string]*discordgo.Role{}

	if len(adminRoles) == 0 {
		log.Println("no admin roles configured")
		return allowedRoles
	}

	for _, allowedRoleName := range adminRoles {
		if r, ok := guildRoles[allowedRoleName]; ok {
			allowedRoles[r.ID] = r
		}
	}

	return allowedRoles
}
