package musicplugin

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/iopred/bruxism"
)

const commandName = "tunes"

type MusicPlugin struct {
	sync.Mutex

	discord          *bruxism.Discord
	VoiceConnections map[string]*voiceConnection
	adminRoles       map[string][]string // guild id -> role names
}

type voiceConnection struct {
	sync.Mutex
	debug bool

	GuildID      string
	ChannelID    string
	MaxQueueSize int
	Queue        []song

	close   chan struct{}
	control chan controlMessage
	playing *song
	conn    *discordgo.VoiceConnection
}

type controlMessage int

const (
	Skip controlMessage = iota
	Pause
	Resume
)

type song struct {
	AddedBy     string
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	FullTitle   string `json:"full_title"`
	Thumbnail   string `json:"thumbnail"`
	URL         string `json:"webpage_url"`
	Duration    int    `json:"duration"`
	Remaining   int
}

func (s song) String() string {
	return fmt.Sprintf("Title: %s, ID: %s, AddedBy: %s", s.Title, s.ID, s.AddedBy)
}

// New will create a new music plugin.
func New(discord *bruxism.Discord, adminRoles map[string][]string) bruxism.Plugin {

	p := &MusicPlugin{
		discord:          discord,
		VoiceConnections: make(map[string]*voiceConnection),
		adminRoles:       adminRoles,
	}

	return p
}

// Name returns the name of the plugin.
func (p *MusicPlugin) Name() string {
	return commandName
}

// Load will load plugin state from a byte array.
func (p *MusicPlugin) Load(bot *bruxism.Bot, service bruxism.Service, data []byte) (err error) {
	if service.Name() != bruxism.DiscordServiceName {
		panic("Tunes Plugin only supports Discord.")
	}

	if data != nil {
		if err = json.Unmarshal(data, p); err != nil {
			log.Println("tunesplugin: loading data err:", err)
		}
	}

	go p.init()

	return nil
}

func (p *MusicPlugin) init() {
	<-time.After(1 * time.Second)
	for _, s := range p.discord.Sessions {
		if !s.DataReady {
			go p.init()
			return
		}
	}
	p.ready()
}

func (p *MusicPlugin) ready() {
	// Join all registered voice channels and start the playback queue
	for _, v := range p.VoiceConnections {
		if v.ChannelID == "" {
			continue
		}
		vc, err := p.join(v.ChannelID)
		if err != nil {
			log.Println("tunesplugin: join channel err:", err)
			continue
		}
		p.gostart(vc)
	}
}

// Save will save plugin state to a byte array.
func (p *MusicPlugin) Save() ([]byte, error) {
	return json.Marshal(p)
}

// Help returns a list of help strings that are printed when the user requests them.
func (p *MusicPlugin) Help(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message, detailed bool) []string {
	help := []string{
		bruxism.CommandHelp(service, commandName, "<command>", "Tunes Plugin, see `help tunes`")[0],
	}

	if detailed {
		help = append(help, []string{
			"Examples:",
			bruxism.CommandHelp(service, commandName, "join [channelid]", "Join your voice channel or the provided voice channel.")[0],
			bruxism.CommandHelp(service, commandName, "leave", "Leave current voice channel.")[0],
			bruxism.CommandHelp(service, commandName, "play [song name]", "Start playing music and optionally enqueue a song by name.")[0],
			bruxism.CommandHelp(service, commandName, "add [URL]", "Start playing music and optionally enqueue a song by URL.")[0],
			bruxism.CommandHelp(service, commandName, "info", "Information about this plugin and the currently playing song.")[0],
			bruxism.CommandHelp(service, commandName, "pause", "Pause playback of current song.")[0],
			bruxism.CommandHelp(service, commandName, "resume", "Resume playback of current song.")[0],
			bruxism.CommandHelp(service, commandName, "skip", "Skip current song.")[0],
			bruxism.CommandHelp(service, commandName, "stop", "Stop playing music.")[0],
			bruxism.CommandHelp(service, commandName, "list", "List contents of queue.")[0],
			bruxism.CommandHelp(service, commandName, "clear", "Clear all items from queue.")[0],
		}...)
	}

	return help
}

// Message handler.
func (p *MusicPlugin) Message(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message) {
	defer bruxism.MessageRecover()

	if service.IsMe(message) {
		return
	}

	if message.Type() == bruxism.MessageTypeUpdate {
		return
	}

	if !bruxism.MatchesCommand(service, commandName, message) && !bruxism.MatchesCommand(service, "tu", message) {
		return
	}

	if service.IsPrivate(message) {
		service.SendMessage(message.Channel(), "Sorry, this command doesn't work in private chat.")
		return
	}

	_, parts := bruxism.ParseCommand(service, message)

	if len(parts) == 0 {
		service.SendMessage(message.Channel(), strings.Join(p.Help(bot, service, message, true), "\n"))
		return
	}

	// Get the Channel (and GuildID) for this channel because it's needed in
	// a few locations below
	channel, err := p.discord.Channel(message.Channel())
	if err != nil {
		log.Println("tunesplugin: fetching channel err:", err.Error())
		return
	}

	// grab pointer to this channels voice connection, if exists.
	vc, vcok := p.VoiceConnections[channel.GuildID]

	switch parts[0] {

	case "help":
		// display extended help information
		service.SendMessage(message.Channel(), strings.Join(p.Help(bot, service, message, true), "\n"))

	case "stats":
		// TODO: maybe provide plugin stats, total channels, total song queues, etc

	case "join":
		// join the voice channel of the caller or the provided channel ID

		channelID := ""
		if len(parts) > 1 {
			channelID = parts[1]
		}

		if channelID == "" {
			messageUserID := message.UserID()
			for _, g := range p.discord.Guilds() {
				for _, v := range g.VoiceStates {
					if v.UserID == messageUserID {
						channelID = v.ChannelID
					}
				}
			}

			if channelID == "" {
				service.SendMessage(message.Channel(), "I couldn't find you in any voice channels, please join one.")
				return
			}
		}

		_, err := p.join(channelID)
		if err != nil {
			service.SendMessage(message.Channel(), err.Error())
			break
		}

		service.SendMessage(message.Channel(), "Now, let's play some tunes!")

	case "leave":
		if !vcok {
			service.SendMessage(message.Channel(), "There is no voice connection for this Guild.")
		}

		vc.conn.Disconnect()
		vc.conn.Close()
		delete(p.VoiceConnections, channel.GuildID)
		service.SendMessage(message.Channel(), "Closed voice connection.")

	case "debug":
		// enable or disable debug

		if !vcok {
			service.SendMessage(message.Channel(), fmt.Sprintf("There is no voice connection for this Guild."))
		}

		vc.Lock()
		vc.debug = !vc.debug
		service.SendMessage(message.Channel(), fmt.Sprintf("debug mode set to %v", vc.debug))
		vc.Unlock()

	case "add":
		// Start queue player and optionally enqueue provided songs

		p.gostart(vc)

		for _, v := range parts[1:] {
			url, err := url.Parse(v) // doesn't check much..
			if err != nil {
				continue
			}
			err = p.enqueue(vc, url.String(), service, message)
			if err != nil {
				// TODO: Might need improving.
				service.SendMessage(message.Channel(), err.Error())
			}
		}

	case "play":
		p.gostart(vc)

		songName := strings.Join(parts[1:], " ")

		if len(songName) == 0 {
			service.SendMessage(message.Channel(), "Please give me the name of the song. `play <song name>`")
		}
		err = p.enqueue(vc, "ytsearch:"+songName, service, message)
		if err != nil {
			service.SendMessage(message.Channel(), err.Error())
		}

	case "stop":
		// stop the queue player

		if !vcok {
			service.SendMessage(message.Channel(), fmt.Sprintf("Can't stop if i am not doing anything. :taps_head:"))
			return
		}

		if vc.close != nil {
			close(vc.close)
			vc.close = nil
		}

		if vc.control != nil {
			close(vc.control)
			vc.control = nil
		}

	case "skip":
		// skip current song

		if vc.control == nil {
			return
		}
		vc.control <- Skip

	case "pause":
		// pause the queue player
		if vc.control == nil {
			return
		}
		vc.control <- Pause

	case "resume":
		// resume the queue player
		if vc.control == nil {
			return
		}
		vc.control <- Resume

	case "info":
		// report player settings, queue info, and current song

		msg := fmt.Sprintf("`Voldy's awesome TunesPlugin:`\n")
		msg += fmt.Sprintf("`Voice Channel:` %s\n", vc.ChannelID)
		msg += fmt.Sprintf("`Queue Size:` %d\n", len(vc.Queue))

		if vc.playing == nil {
			service.SendMessage(message.Channel(), msg)
			break
		}

		msg += fmt.Sprintf("`Now Playing:`\n")
		msg += fmt.Sprintf("`ID:` %s\n", vc.playing.ID)
		msg += fmt.Sprintf("`Title:` %s\n", vc.playing.Title)
		msg += fmt.Sprintf("`Duration:` %ds\n", vc.playing.Duration)
		msg += fmt.Sprintf("`Remaining:` %ds\n", vc.playing.Remaining)
		msg += fmt.Sprintf("`Source URL:` <%s>\n", vc.playing.URL)
		msg += fmt.Sprintf("`Thumbnail:` %s\n", vc.playing.Thumbnail)
		service.SendMessage(message.Channel(), msg)

	case "list":
		// list top items in the queue

		if vc == nil {
			return
		}

		if len(vc.Queue) == 0 {
			service.SendMessage(message.Channel(), "The tunes queue is empty.")
			return
		}

		var msg string

		i := 1
		i2 := 0
		msg = fmt.Sprintf("Total Songs: %d\n", len(vc.Queue))
		for k, v := range vc.Queue {
			np := ""
			if vc.playing != nil && *vc.playing == v {
				np = "**(Now Playing)**"
			}
			d := time.Duration(v.Duration) * time.Second
			msg += fmt.Sprintf("`%.3d:%.15s` **%s** [%s] - *%s* %s\n", k, v.ID, v.Title, d.String(), v.AddedBy, np)

			if i >= 5 {
				service.SendMessage(message.Channel(), msg)
				msg = ""
				i = 0
				i2++

				if i2 >= 2 {
					// limit response to 8 messages (120 songs)
					return
				}
			}
			i++
		}

		service.SendMessage(message.Channel(), msg)

	case "clear":
		// clear all items from the queue
		vc.Lock()
		vc.Queue = []song{}
		vc.Unlock()

	default:
		service.SendMessage(message.Channel(), "Unknown tunes command, try `help tunes`")
	}
}

// join a specific voice channel
func (p *MusicPlugin) join(cID string) (vc *voiceConnection, err error) {

	c, err := p.discord.Channel(cID)
	if err != nil {
		return
	}

	if c.Type != discordgo.ChannelTypeGuildVoice {
		err = fmt.Errorf("that's not a voice channel")
		return
	}

	// Get or Create the VoiceConnection object
	p.Lock()
	vc, ok := p.VoiceConnections[c.GuildID]
	if !ok {
		vc = &voiceConnection{}
		p.VoiceConnections[c.GuildID] = vc
	}
	p.Unlock()

	guild, err := p.discord.Guild(c.GuildID)
	if err != nil {
		return
	}

	guildID, err := strconv.ParseInt(guild.ID, 10, 64)
	if err != nil {
		return
	}

	shardID := int((guildID >> 22) % int64(len(p.discord.Sessions)))

	// NOTE: Setting mute to false, deaf to true.
	vc.conn, err = p.discord.Sessions[int(shardID)].ChannelVoiceJoin(c.GuildID, cID, false, true)
	if err != nil {
		return
	}

	vc.GuildID = c.GuildID
	vc.ChannelID = cID

	return
}

// enqueue a song/playlest to a VoiceConnections Queue
func (p *MusicPlugin) enqueue(vc *voiceConnection, url string, service bruxism.Service, message bruxism.Message) (err error) {

	if vc == nil {
		return fmt.Errorf("cannot enqueue to nil voice connection")
	}

	if url == "" {
		return fmt.Errorf("cannot enqueue an empty string")
	}

	// TODO //////////////////////////////////////////////////////////////////
	// need to parse the url and have a way to know what we're doing
	// 1) option to queue local files
	// 2) option to queue saved playlists
	// 3) option to queue URL that can be passed directly to ffmpeg without youtube-dl
	// 4) option to queue youtube-dl playlist
	// 5) option to queue youtube-dl song
	// 6) option to queue youtube-dl search result

	// right now option 4 and 5 work, only.
	//////////////////////////////////////////////////////////////////////////

	cmd := exec.Command("./youtube-dl", "-i", "-j", url)
	if vc.debug {
		cmd.Stderr = os.Stderr
	}

	output, err := cmd.StdoutPipe()
	if err != nil {
		log.Println(err)
		service.SendMessage(message.Channel(), fmt.Sprintf("Error adding song to playlist."))
		return
	}

	err = cmd.Start()
	if err != nil {
		log.Println(err)
		service.SendMessage(message.Channel(), fmt.Sprintf("Error adding song to playlist."))
		return
	}
	go func() {
		cmd.Wait()
	}()

	scanner := bufio.NewScanner(output)

	totalAdded := 0
	var firstSong *song
	for scanner.Scan() {
		s := song{}
		err = json.Unmarshal(scanner.Bytes(), &s)
		if err != nil {
			log.Println(err)
			continue
		}

		s.AddedBy = message.UserName()

		if totalAdded == 0 {
			firstSong = &s
		}
		vc.Lock()
		vc.Queue = append(vc.Queue, s)
		vc.Unlock()
		totalAdded++
	}

	updatedQueueMessage := fmt.Sprintf("Added song: %s", firstSong.Title)
	if totalAdded > 1 {
		updatedQueueMessage += fmt.Sprintf(". and %d other.", totalAdded)
	}

	service.SendMessage(message.Channel(), updatedQueueMessage)
	return
}

// little wrapper function for start() to fire it off in a
// go routine if it is not already running.
func (p *MusicPlugin) gostart(vc *voiceConnection) (err error) {

	vc.Lock()

	if vc == nil {
		vc.Unlock()
		return fmt.Errorf("gostart cannot start a nil voice connection queue")
	}

	if vc.close != nil || vc.control != nil {
		vc.Unlock()
		return fmt.Errorf("gostart will not start a voice connection with non-nil control channels")
	}

	vc.close = make(chan struct{})
	vc.control = make(chan controlMessage)

	// TODO can this be moved lower?
	vc.Unlock()

	go p.start(vc, vc.close, vc.control)

	return
}

// "start" is a goroutine function that loops though the music queue and
// plays songs as they are added
func (p *MusicPlugin) start(vc *voiceConnection, close <-chan struct{}, control <-chan controlMessage) {

	if close == nil || control == nil || vc == nil {
		log.Println("tunesplugin: start() exited due to nil channels")
		return
	}

	var i int
	var Song song

	// main loop keeps this going until close
	for {

		// exit if close channel is closed
		select {
		case <-close:
			log.Println("tunesplugin: start() exited due to close channel.")
			return
		default:
		}

		// loop until voice connection is ready and songs are in the queue.
		if vc.conn == nil || vc.conn.Ready == false || len(vc.Queue) < 1 {
			time.Sleep(1 * time.Second)
			continue
		}

		// Get song to play and store it in local Song var
		vc.Lock()
		if len(vc.Queue)-1 >= i {
			Song = vc.Queue[i]
		} else {
			i = 0
			vc.Unlock()
			continue
		}
		vc.Unlock()

		vc.playing = &Song
		p.play(vc, close, control, Song)
		vc.playing = nil

		vc.Lock()
		if len(vc.Queue) > 0 {
			vc.Queue = append(vc.Queue[:i], vc.Queue[i+1:]...)
		}
		vc.Unlock()
	}
}

// play an individual song
func (p *MusicPlugin) play(vc *voiceConnection, close <-chan struct{}, control <-chan controlMessage, s song) {
	var err error

	if close == nil || control == nil || vc == nil || vc.conn == nil {
		log.Println("tunesplugin: play exited because [close|control|vc|vc.conn] is nil.")
		return
	}

	ytdl := exec.Command("./youtube-dl", "-v", "-f", "bestaudio", "-o", "-", s.URL)
	if vc.debug {
		ytdl.Stderr = os.Stderr
	}
	ytdlout, err := ytdl.StdoutPipe()
	if err != nil {
		log.Println("tunesplugin: ytdl StdoutPipe err:", err)
		return
	}
	ytdlbuf := bufio.NewReaderSize(ytdlout, 16384)

	ffmpeg := exec.Command("ffmpeg", "-i", "pipe:0", "-f", "s16le", "-ar", "48000", "-ac", "2", "-af", "volume=0.5", "pipe:1")
	ffmpeg.Stdin = ytdlbuf
	if vc.debug {
		ffmpeg.Stderr = os.Stderr
	}
	ffmpegout, err := ffmpeg.StdoutPipe()
	if err != nil {
		log.Println("tunesplugin: ffmpeg StdoutPipe err:", err)
		return
	}
	ffmpegbuf := bufio.NewReaderSize(ffmpegout, 16384)

	dca := exec.Command("./dca")
	dca.Stdin = ffmpegbuf
	if vc.debug {
		dca.Stderr = os.Stderr
	}
	dcaout, err := dca.StdoutPipe()
	if err != nil {
		log.Println("tunesplugin: dca StdoutPipe err:", err)
		return
	}
	dcabuf := bufio.NewReaderSize(dcaout, 16384)

	err = ytdl.Start()
	if err != nil {
		log.Println("tunesplugin: ytdl Start err:", err)
		return
	}
	go func() {
		ytdl.Wait()
	}()

	err = ffmpeg.Start()
	if err != nil {
		log.Println("tunesplugin: ffmpeg Start err:", err)
		return
	}
	go func() {
		ffmpeg.Wait()
	}()

	err = dca.Start()
	if err != nil {
		log.Println("tunesplugin: dca Start err:", err)
		return
	}
	go func() {
		dca.Wait()
	}()

	// header "buffer"
	var opuslen int16

	// Send "speaking" packet over the voice websocket
	vc.conn.Speaking(true)

	// Send not "speaking" packet over the websocket when we finish
	defer vc.conn.Speaking(false)

	start := time.Now()
	for {

		select {
		case <-close:
			log.Println("tunesplugin: play() exited due to close channel.")
			return
		default:
		}

		select {
		case ctl := <-control:
			switch ctl {
			case Skip:
				return
				break
			case Pause:
				done := false
				for {

					ctl, ok := <-control
					if !ok {
						return
					}
					switch ctl {
					case Skip:
						return
						break
					case Resume:
						done = true
						break
					}

					if done {
						break
					}

				}
			}
		default:
		}

		// read dca opus length header
		err = binary.Read(dcabuf, binary.LittleEndian, &opuslen)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return
		}
		if err != nil {
			log.Println("tunesplugin: read opus length from dca err:", err)
			return
		}

		// read opus data from dca
		opus := make([]byte, opuslen)
		err = binary.Read(dcabuf, binary.LittleEndian, &opus)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return
		}
		if err != nil {
			log.Println("tunesplugin: read opus from dca err:", err)
			return
		}

		// Send received PCM to the sendPCM channel
		vc.conn.OpusSend <- opus
		// TODO: Add a select and timeout to above
		// shouldn't ever block longer than maybe 18-25ms

		// this can cause a panic if vc becomes nil while waiting to send
		// on the opus channel. TODO fix..
		vc.playing.Remaining = (vc.playing.Duration - int(time.Since(start).Seconds()))

	}
}

// Stats will return the stats for a plugin.
func (p *MusicPlugin) Stats(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message) []string {
	return nil
}

func (p *MusicPlugin) isUserAdmin(guildID, userID string) bool {
	roles, ok := p.adminRoles[guildID]
	if !ok {
		return false
	}

	guildRoles := fetchGuildRoles(p.discord.Session, guildID)
	allowedRoles := filterRoles(guildRoles, roles, guildID)

	guildMember, err := p.discord.Session.GuildMember(guildID, userID)
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
