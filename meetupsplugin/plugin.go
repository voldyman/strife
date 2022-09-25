package meetupsplugin

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/dustin/go-humanize"
	"github.com/iopred/bruxism"
)

// MeetupsPlugin is a plugin that reminds users.
type MeetupsPlugin struct {
	sync.RWMutex
	bot     *bruxism.Bot
	discord *bruxism.Discord
}

const serverMeetupsChannelID = "680975706372833280" // #meetups

// Help returns a list of help strings that are printed when the user requests them.
func (p *MeetupsPlugin) Help(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message, detailed bool) []string {
	help := []string{
		bruxism.CommandHelp(service, "reminder", "<time> <reminder>", "Sets a reminder that is sent after the provided time.")[0],
	}
	return help
}

func (p *MeetupsPlugin) Message(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message) {
	defer bruxism.MessageRecover()

	if service.IsMe(message) {
		return
	}

	if message.Channel() != serverMeetupsChannelID {
		return
	}
	fmt.Println("got message in channel", message.Channel())
	ch, err := p.discord.Session.Channel(message.Channel())
	if err != nil {
		fmt.Println("unable to get channel details", err)
		return
	}
	fmt.Println("channel name:", ch.Name)

}

// Load will load plugin state from a byte array.
func (p *MeetupsPlugin) Load(bot *bruxism.Bot, service bruxism.Service, data []byte) error {
	if data != nil {
		if err := json.Unmarshal(data, p); err != nil {
			log.Println("Error loading data", err)
		}
	}

	return nil
}

// Save will save plugin state to a byte array.
func (p *MeetupsPlugin) Save() ([]byte, error) {
	return json.Marshal(p)
}

// Stats will return the stats for a plugin.
func (p *MeetupsPlugin) Stats(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message) []string {
	return []string{fmt.Sprintf("Meetups: \t%s\n", humanize.Comma(int64(0)))}
}

// Name returns the name of the plugin.
func (p *MeetupsPlugin) Name() string {
	return "Reminder"
}

// New will create a new Reminder plugin.
func New(discord *bruxism.Discord) bruxism.Plugin {
	return &MeetupsPlugin{
		discord: discord,
	}
}
