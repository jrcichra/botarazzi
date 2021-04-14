package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

var (
	//Token is the discord token for this bot
	Token string
	//Hostname is where the link will give botarazzi for downloads
	Hostname string
	//VoiceConnections -
	VoiceConnections map[string]chan struct{}
	//Speaker Streams - takes an ssrc and returns a userid
	SpeakerStreams map[int]string
)

func init() {
	VoiceConnections = make(map[string]chan struct{})
	SpeakerStreams = make(map[int]string)
	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.StringVar(&Hostname, "h", "localhost", "Hostname to serve zips")
	flag.Parse()
}

func main() {
	// serve an http server for downloading zips
	go serve()
	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}
	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)
	// Look at voice state changes
	dg.AddHandler(voiceStateUpdate)
	// Get all intents
	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsAllWithoutPrivileged)

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}
