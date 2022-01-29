package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"github.com/iopred/bruxism"
	"github.com/iopred/bruxism/chartplugin"
	"github.com/iopred/bruxism/discordavatarplugin"
	"github.com/iopred/bruxism/emojiplugin"
	"github.com/iopred/bruxism/inviteplugin"
	"github.com/iopred/bruxism/mysonplugin"
	"github.com/iopred/bruxism/numbertriviaplugin"
	"github.com/iopred/bruxism/playedplugin"
	"github.com/iopred/bruxism/playingplugin"
	"github.com/iopred/bruxism/statsplugin"
	"github.com/iopred/bruxism/triviaplugin"
	"github.com/voldyman/strife/musicplugin"
	"github.com/voldyman/strife/reminderplugin"
	"github.com/voldyman/strife/welcomeplugin"
)

var discordToken string
var discordEmail string
var discordPassword string
var discordApplicationClientID string
var discordOwnerUserID string
var imgurID string
var imgurAlbum string
var mashableKey string
var adminMapping = map[string][]string{
	"707620933841453186": {"Bot man", "I hear voices", "Music"}, // "bot man", "i hear voices" and "music"
}

func init() {
	flag.StringVar(&discordToken, "discordtoken", "", "Discord token.")
	flag.StringVar(&discordEmail, "discordemail", "", "Discord account email.")
	flag.StringVar(&discordPassword, "discordpassword", "", "Discord account password.")
	flag.StringVar(&discordOwnerUserID, "discordowneruserid", "", "Discord owner user id.")
	flag.StringVar(&discordApplicationClientID, "discordapplicationclientid", "", "Discord application client id.")
	flag.StringVar(&imgurID, "imgurid", "", "Imgur client id.")
	flag.StringVar(&imgurAlbum, "imguralbum", "", "Imgur album id.")
	flag.StringVar(&mashableKey, "mashablekey", "", "Mashable key.")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())
}

func main() {
	q := make(chan bool)

	// Set our variables.
	bot := bruxism.NewBot()
	bot.ImgurID = imgurID
	bot.ImgurAlbum = imgurAlbum
	bot.MashableKey = mashableKey

	// Generally CommandPlugins don't hold state, so we share one instance of the command plugin for all services.
	cp := bruxism.NewCommandPlugin()
	cp.AddCommand("invite", inviteplugin.InviteCommand, inviteplugin.InviteHelp)
	cp.AddCommand("join", inviteplugin.InviteCommand, nil)
	cp.AddCommand("stats", statsplugin.StatsCommand, statsplugin.StatsHelp)
	cp.AddCommand("info", statsplugin.StatsCommand, nil)
	cp.AddCommand("stat", statsplugin.StatsCommand, nil)
	if bot.MashableKey != "" {
		cp.AddCommand("numbertrivia", numbertriviaplugin.NumberTriviaCommand, numbertriviaplugin.NumberTriviaHelp)
	}
	cp.AddCommand("quit", func(bot *bruxism.Bot, service bruxism.Service, message bruxism.Message, args string, parts []string) {
		if service.IsBotOwner(message) {
			q <- true
		}
	}, nil)

	// Register the Discord service if we have an email or token.
	credsMissing := (discordEmail == "" && discordPassword == "")
	if credsMissing && discordToken == "" {
		fmt.Println("please specify discord email & password or discord token to run the bot")
		os.Exit(1)
	}
	var discord *bruxism.Discord
	if discordToken != "" {
		discord = bruxism.NewDiscord(discordToken)
	} else {
		discord = bruxism.NewDiscord(discordEmail, discordPassword)
	}
	discord.ApplicationClientID = discordApplicationClientID
	discord.OwnerUserID = discordOwnerUserID

	bot.RegisterService(discord)
	bot.RegisterPlugin(discord, cp)
	bot.RegisterPlugin(discord, chartplugin.New())
	bot.RegisterPlugin(discord, discordavatarplugin.New())
	bot.RegisterPlugin(discord, emojiplugin.New())
	bot.RegisterPlugin(discord, musicplugin.New(discord, adminMapping))
	bot.RegisterPlugin(discord, mysonplugin.New())
	bot.RegisterPlugin(discord, playedplugin.New())
	bot.RegisterPlugin(discord, playingplugin.New())
	bot.RegisterPlugin(discord, reminderplugin.New())
	bot.RegisterPlugin(discord, triviaplugin.New())
	bot.RegisterPlugin(discord, welcomeplugin.New(discord, discordOwnerUserID))

	bot.Open()

	// Wait for a termination signal, while saving the bot state every minute. Save on close.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)

	t := time.Tick(1 * time.Minute)

out:
	for {
		select {
		case <-q:
			break out
		case <-c:
			break out
		case <-t:
			bot.Save()
		}
	}

	bot.Save()
}
