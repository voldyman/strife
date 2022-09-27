package reminderplugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/iopred/bruxism"
	"github.com/tj/go-naturaldate"
)

// A Reminder holds data about a specific reminder.
type Reminder struct {
	StartTime time.Time
	Time      time.Time
	Requester string
	Target    string
	Message   string
	IsPrivate bool
}

// ReminderPlugin is a plugin that reminds users.
type ReminderPlugin struct {
	sync.RWMutex
	bot            *bruxism.Bot
	discord        *bruxism.Discord
	Reminders      []*Reminder
	TotalReminders int
}

var randomTimes = []string{
	"1 minute",
	"10 minutes",
	"1 hour",
	"4 hours",
	"tomorrow",
	"next week",
}

var randomMessages = []string{
	"walk the dog",
	"take pizza out of the oven",
	"check my email",
	"feed the baby",
	"play some quake",
}

func (p *ReminderPlugin) random(list []string) string {
	return list[rand.Intn(len(list))]
}

func (p *ReminderPlugin) randomReminder(service bruxism.Service) string {
	ticks := ""
	if service.Name() == bruxism.DiscordServiceName {
		ticks = "`"
	}

	return fmt.Sprintf("%s%sreminder %s %s%s", ticks, service.CommandPrefix(), p.random(randomTimes), p.random(randomMessages), ticks)
}

// Help returns a list of help strings that are printed when the user requests them.
func (p *ReminderPlugin) Help(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message, detailed bool) []string {
	help := []string{
		bruxism.CommandHelp(service, "reminder", "<time> <reminder>", "Sets a reminder that is sent after the provided time.")[0],
		bruxism.CommandHelp(service, "reminderlist", "", "List all active reminders.")[0],
		bruxism.CommandHelp(service, "reminderdelete", "<index>", "Deletes a reminder by index. eg: reminderdelete 0")[0],
	}
	if detailed {
		help = append(help, []string{
			"Examples: ",
			p.randomReminder(service),
			p.randomReminder(service),
		}...)
	}
	return help
}

func (p *ReminderPlugin) parseReminder(parts []string) (time.Time, string, error) {
	if parts[0] == "me" {
		parts = parts[1:]
	}
	if len(parts) < 2 {
		return time.Time{}, "", fmt.Errorf("too few parts to parse: %+v", parts)
	}
	if parts[0] == "tomorrow" {
		return time.Now().Add(1 * time.Hour * 24), strings.Join(parts[1:], " "), nil
	}

	if parts[0] == "next" {
		switch parts[1] {
		case "week":
			return time.Now().Add(1 * time.Hour * 24 * 7), strings.Join(parts[2:], " "), nil
		case "month":
			return time.Now().Add(1 * time.Hour * 24 * 7 * 4), strings.Join(parts[2:], " "), nil
		case "year":
			return time.Now().Add(1 * time.Hour * 24 * 365), strings.Join(parts[2:], " "), nil
		default:
			return time.Time{}, "", errors.New("Invalid next.")
		}
	}

	i, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, "", err
	}

	switch {
	case strings.HasPrefix(parts[1], "sec"):
		return time.Now().Add(time.Duration(i) * time.Second), strings.Join(parts[2:], " "), nil
	case strings.HasPrefix(parts[1], "min"):
		return time.Now().Add(time.Duration(i) * time.Minute), strings.Join(parts[2:], " "), nil
	case strings.HasPrefix(parts[1], "hour"):
		return time.Now().Add(time.Duration(i) * time.Hour), strings.Join(parts[2:], " "), nil
	case strings.HasPrefix(parts[1], "day"):
		return time.Now().Add(time.Duration(i) * time.Hour * 24), strings.Join(parts[2:], " "), nil
	case strings.HasPrefix(parts[1], "week"):
		return time.Now().Add(time.Duration(i) * time.Hour * 24 * 7), strings.Join(parts[2:], " "), nil
	case strings.HasPrefix(parts[1], "month"):
		return time.Now().Add(time.Duration(i) * time.Hour * 24 * 31), strings.Join(parts[2:], " "), nil
	case strings.HasPrefix(parts[1], "year"):
		return time.Now().Add(time.Duration(i) * time.Hour * 24 * 365), strings.Join(parts[2:], " "), nil
	}

	return time.Time{}, "", errors.New("Invalid string.")
}

// AddReminder adds a reminder.
func (p *ReminderPlugin) AddReminder(reminder *Reminder) error {
	p.Lock()
	defer p.Unlock()

	i := 0
	for _, r := range p.Reminders {
		if r.Requester == reminder.Requester {
			i++
			if i > 5 {
				return errors.New("You have too many reminders already.")
			}
		}
	}

	i = 0
	for _, r := range p.Reminders {
		if r.Time.After(reminder.Time) {
			break
		}
		i++
	}

	p.Reminders = append(p.Reminders, reminder)
	copy(p.Reminders[i+1:], p.Reminders[i:])
	p.Reminders[i] = reminder
	p.TotalReminders++

	return nil
}

func (p *ReminderPlugin) Message(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message) {
	defer bruxism.MessageRecover()

	if service.IsMe(message) {
		return
	}

	requester := message.UserName()
	if service.Name() == bruxism.DiscordServiceName {
		requester = fmt.Sprintf("<@%s>", message.UserID())
	}

	if bruxism.MatchesCommand(service, "remindlist", message) || bruxism.MatchesCommand(service, "reminderlist", message) {
		reminders := []string{}
		i := 0
		for _, r := range p.Reminders {
			if r.Requester == requester {
				reminders = append(reminders, fmt.Sprintf("%d - %s: %s", i, humanize.Time(r.Time), r.Message))
				i++
			}
		}
		if len(reminders) > 0 {
			if service.SupportsMultiline() {
				service.SendMessage(message.Channel(), fmt.Sprintf("Your reminders:\n%s", strings.Join(reminders, "\n")))
			} else {
				service.SendMessage(message.Channel(), fmt.Sprintf("Your reminders: %s", strings.Join(reminders, ". ")))
			}
		} else {
			service.SendMessage(message.Channel(), "You have no reminders.")
		}
		return
	}

	if bruxism.MatchesCommand(service, "reminddelete", message) || bruxism.MatchesCommand(service, "reminderdelete", message) {
		index := 0
		indexString, parts := bruxism.ParseCommand(service, message)
		var err error
		if len(parts) > 0 {
			index, err = strconv.Atoi(indexString)
			if err != nil {
				service.SendMessage(message.Channel(), "Invalid reminder.")
				return
			}
		}

		j := 0
		for i, r := range p.Reminders {
			if r.Requester == requester {
				if j == index {
					p.Lock()
					p.Reminders = append(p.Reminders[:i], p.Reminders[i+1:]...)
					p.Unlock()
					service.SendMessage(message.Channel(), "Reminder deleted.")
					return
				}
				j++
			}
		}
		return
	}

	if !bruxism.MatchesCommand(service, "remind", message) && !bruxism.MatchesCommand(service, "reminder", message) {
		return
	}

	_, parts := bruxism.ParseCommand(service, message)

	if len(parts) < 2 {
		service.SendMessage(message.Channel(), fmt.Sprintf("Invalid reminder, no time or message. eg: %s", p.randomReminder(service)))
		return
	}

	t, r, err := p.parseReminder(parts)

	now := time.Now()

	if err != nil || t.Before(now) || t.After(now.Add(time.Hour*24*365+time.Hour)) {
		service.SendMessage(message.Channel(), fmt.Sprintf("Invalid time. eg: %s", strings.Join(randomTimes, ", ")))
		return
	}

	if r == "" {
		service.SendMessage(message.Channel(), fmt.Sprintf("Invalid reminder, no message. eg: %s", p.randomReminder(service)))
		return
	}

	if service.Name() == bruxism.DiscordServiceName {
		if strings.Index(r, "<@") != -1 || strings.Index(strings.ToLower(r), "@everyone") != -1 || strings.Index(strings.ToLower(r), "@here") != -1 {
			service.SendMessage(message.Channel(), "Invalid reminder, no mentions, sorry.")
			return
		}
	}

	t = t.Add(500 * time.Millisecond)

	err = p.AddReminder(&Reminder{
		StartTime: now,
		Time:      t,
		Requester: requester,
		Target:    message.Channel(),
		Message:   r,
		IsPrivate: service.IsPrivate(message),
	})
	if err != nil {
		service.SendMessage(message.Channel(), err.Error())
		return
	}

	service.SendMessage(message.Channel(), fmt.Sprintf("Reminder set for %s.", humanize.Time(t)))
}

// SendReminder sends a reminder.
func (p *ReminderPlugin) SendReminder(service bruxism.Service, reminder *Reminder) {
	if strings.Index(reminder.Message, "<@") != -1 || strings.Index(strings.ToLower(reminder.Message), "@everyone") != -1 || strings.Index(strings.ToLower(reminder.Message), "@here") != -1 {
		return
	}

	if reminder.IsPrivate {
		service.SendMessage(reminder.Target, fmt.Sprintf("%s you set a reminder: %s", humanize.Time(reminder.StartTime), reminder.Message))
	} else {
		service.SendMessage(reminder.Target, fmt.Sprintf("%s %s set a reminder: %s", humanize.Time(reminder.StartTime), reminder.Requester, reminder.Message))
	}
}

// Run will block until a reminder needs to be fired and then fire it.
func (p *ReminderPlugin) Run(bot *bruxism.Bot, service bruxism.Service) {
	for {
		p.RLock()

		if len(p.Reminders) > 0 {
			reminder := p.Reminders[0]
			if time.Now().After(reminder.Time) {
				p.RUnlock()
				if time.Now().Before(reminder.Time.Add(48 * time.Hour)) {
					p.SendReminder(service, reminder)
				}
				p.Lock()
				p.Reminders = p.Reminders[1:]
				p.Unlock()

				continue
			}
		}

		p.RUnlock()
		time.Sleep(500 * time.Millisecond)
	}
}

// Load will load plugin state from a byte array.
func (p *ReminderPlugin) Load(bot *bruxism.Bot, service bruxism.Service, data []byte) error {
	if data != nil {
		if err := json.Unmarshal(data, p); err != nil {
			log.Println("Error loading data", err)
		}
	}
	if len(p.Reminders) > p.TotalReminders {
		p.TotalReminders = len(p.Reminders)
	}

	for _, s := range p.discord.Sessions {
		for _, guild := range s.State.Guilds {
			cmd, err := p.discord.Session.ApplicationCommandCreate(p.discord.Session.State.User.ID, guild.ID, createReminderCMD())
			if err != nil {
				log.Print("unable to create command:", err)
				continue
			}
			log.Print("created remindme command:", cmd.ApplicationID, "for guild:", guild.Name)
		}
		p.discord.Session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			if i.ApplicationCommandData().Name == "remindme" {
				p.handleCreateReminderCMD(s, i)
			}
		})
	}
	go p.Run(bot, service)
	return nil
}

func createReminderCMD() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		ID:          "remdind-me-nalak",
		Name:        "remindme",
		Description: "create a reminder",
		Options: []*discordgo.ApplicationCommandOption{
			{Name: "when", Required: true, Type: discordgo.ApplicationCommandOptionString, Description: "2 days, 1 hour, etc."},
			{Name: "what", Required: true, Type: discordgo.ApplicationCommandOptionString, Description: "What is the reminder message?"},
		},
	}
}
func (p *ReminderPlugin) handleCreateReminderCMD(s *discordgo.Session, i *discordgo.InteractionCreate) {
	what := ""
	when := ""
	for _, opt := range i.ApplicationCommandData().Options {
		if opt.Name == "when" {
			when = opt.StringValue()
		}
		if opt.Name == "what" {
			what = opt.StringValue()
		}
	}
	trigger, err := naturaldate.Parse(when, time.Now(), naturaldate.WithDirection(naturaldate.Future))
	if err != nil {
		p.sendInteractionResponse(s, i, fmt.Sprintf("unable to parse time: '%s'", when))
		return
	}
	now := time.Now()
	if trigger.Before(now) || trigger.After(now.Add(time.Hour*24*365+time.Hour)) {
		fmt.Println("Invalid time", humanize.Time(trigger))
		p.sendInteractionResponse(s, i, fmt.Sprintf("Invalid time. eg: %s", strings.Join(randomTimes, ", ")))
		return
	}

	err = p.AddReminder(&Reminder{
		StartTime: now,
		Time:      trigger,
		Requester: userID(i),
		Target:    i.ChannelID,
		Message:   what,
		IsPrivate: false,
	})
	if err != nil {
		p.sendInteractionResponse(s, i, fmt.Sprintf("error adding reminder: %s", err.Error()))
		return
	}
	p.sendInteractionResponse(s, i, fmt.Sprintf("Added reminder for %s", humanize.Time(trigger)))
}
func userID(i *discordgo.InteractionCreate) string {
	userID := ""
	if i.User != nil {
		userID = i.User.ID
	}
	if i.Member != nil {
		userID = i.Member.User.ID
	}
	return userID
}

func (p *ReminderPlugin) sendInteractionResponse(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
		},
	})
	if err != nil {
		log.Print("unable to respond")
	} else {
		log.Print("successfully responded to command")
	}
}

// Save will save plugin state to a byte array.
func (p *ReminderPlugin) Save() ([]byte, error) {
	return json.Marshal(p)
}

// Stats will return the stats for a plugin.
func (p *ReminderPlugin) Stats(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message) []string {
	return []string{fmt.Sprintf("Reminders: \t%s\n", humanize.Comma(int64(p.TotalReminders)))}
}

// Name returns the name of the plugin.
func (p *ReminderPlugin) Name() string {
	return "Reminder"
}

// New will create a new Reminder plugin.
func New(discord *bruxism.Discord) bruxism.Plugin {
	return &ReminderPlugin{
		Reminders: []*Reminder{},
		discord:   discord,
	}
}
