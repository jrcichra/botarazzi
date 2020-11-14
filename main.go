package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jonas747/dca"

	"github.com/bwmarrin/discordgo"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/oggwriter"
)

var (
	//Token is the discord token for this bot
	Token string
	//VoiceConnections -
	VoiceConnections map[string]chan struct{}
)

func init() {
	VoiceConnections = make(map[string]chan struct{})
	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.Parse()
}

func main() {
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

// This function will be called (due to AddHandler above)
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}
	// If the message is "ping" reply with "Pong!"
	if strings.ToLower(m.Content) == "!ping" {
		s.ChannelMessageSend(m.ChannelID, "Pong!")
	}

	// If the message is "pong" reply with "Ping!"
	if strings.ToLower(m.Content) == "!pong" {
		s.ChannelMessageSend(m.ChannelID, "Ping!")
	}

	if strings.ToLower(m.Content) == "!leave" {
		//Leave the guild's voice if we were in it
		if _, ok := VoiceConnections[m.GuildID]; ok {
			VoiceConnections[m.GuildID] <- struct{}{}
		} else {
			s.ChannelMessageSend(m.ChannelID, "Cannot leave. Bot was not in a voice channel")
		}
	}

	if strings.ToLower(m.Content) == "!join" {
		// Connect to voice channel with user who requested the bot
		channels, err := s.GuildChannels(m.GuildID)
		if err != nil {
			panic(err)
		}
		//grab the user's guild for this run
		guild, err := s.State.Guild(m.GuildID)

		if err != nil {
			panic(err)
		}

		//Locate the channel of the user who requested a join
		found := false
		for _, channel := range channels {
			if channel.Type == discordgo.ChannelTypeGuildVoice {

				vcUserIDs, err := voiceChannelUsers(s, channel.ID, guild, s.VoiceConnections)
				if err != nil {
					panic(err)
				}
				for _, vcUserID := range vcUserIDs {
					//Convert ID to User Object
					//TODO: Remove if object is not used, only ID
					vcUser, err := s.User(vcUserID)
					if err != nil {
						panic(err)
					}
					if vcUser.ID == m.Author.ID {
						found = true
						v, err := s.ChannelVoiceJoin(guild.ID, channel.ID, false, false)
						if err != nil {
							s.ChannelMessageSend(m.ChannelID, "failed to join voice channel: "+err.Error())
							return
						}
						// Add chan to voice channel map
						c := make(chan struct{})
						VoiceConnections[guild.ID] = c
						go handleVoiceChannel(v, c, guild.ID)
					}
				}

			}

		}
		if !found {
			s.ChannelMessageSend(m.ChannelID, "Cannot join. "+m.Author.Username+" is not in a voice channel")
		}
	}
}

func voiceStateUpdate(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {

	fmt.Println("Change in voice state")
	if m.ChannelID == "" { //User disconnected from a voice channel
		println(m.UserID, " left channel ", m.ChannelID)
	} else {
		//We need to capture a join as "this is a user we should record"

	}
}

// VoiceChannelUsers returns IDS of users present in given channelID
func voiceChannelUsers(session *discordgo.Session, channelID string, guild *discordgo.Guild, vcs map[string]*discordgo.VoiceConnection) (st []string, err error) {
	st = make([]string, 0)
	for _, voiceStates := range guild.VoiceStates {
		if channelID == voiceStates.ChannelID {
			st = append(st, voiceStates.UserID)
		}
	}
	return
}

func createPionRTPPacket(p *discordgo.Packet) *rtp.Packet {
	return &rtp.Packet{
		Header: rtp.Header{
			Version: 2,
			// Taken from Discord voice docs
			PayloadType:    0x78,
			SequenceNumber: p.Sequence,
			Timestamp:      p.Timestamp,
			SSRC:           p.SSRC,
		},
		Payload: p.Opus,
	}
}

func handleVoiceChannel(v *discordgo.VoiceConnection, c chan struct{}, gid string) {

	//play a sound when we join
	v.Speaking(true)
	playSound(v)
	v.Speaking(false)

	files := make(map[uint32]media.Writer)
	done := false

	// get date for folder
	d := time.Now().Unix()
	err := os.MkdirAll(fmt.Sprintf("recordings/%d", d), 0755)
	if err != nil {
		panic(err)
	}

	for !done {
		select {
		case p, open := <-v.OpusRecv:
			if open {
				file, ok := files[p.SSRC]
				if !ok {
					var err error
					file, err = oggwriter.New(fmt.Sprintf("recordings/%d/%d.ogg", d, p.SSRC), 48000, 2)
					if err != nil {
						fmt.Printf("failed to create file recordings/%d/%d.ogg, giving up on recording: %v\n", d, p.SSRC, err)
						continue
					}
					files[p.SSRC] = file
				}
				// Construct pion RTP packet from DiscordGo's type.
				rtp := createPionRTPPacket(p)
				err = file.WriteRTP(rtp)
				if err != nil {
					fmt.Printf("failed to write to file recordings/%d/%d.ogg, giving up on recording: %v\n", d, p.SSRC, err)
				}
			} else {
				// Once we made it here, we're done listening for packets. Close all files
				for _, f := range files {
					f.Close()
				}
				//Break the loop
				done = true
			}
		case <-c:
			//close the voiceconnection
			close(v.OpusRecv)
			v.Close()
			v.Disconnect()
			//Remove ourselves from the mapping
			delete(VoiceConnections, gid)
		}
	}

}

func playSound(v *discordgo.VoiceConnection) {
	// Encoding a file and saving it to disk
	encodeSession, err := dca.EncodeFile("welcome.ogg", dca.StdEncodeOptions)
	if err != nil {
		panic(err)
	}
	// Make sure everything is cleaned up, that for example the encoding process if any issues happened isnt lingering around
	defer encodeSession.Cleanup()

	for {
		frame, err := encodeSession.OpusFrame()
		if err != nil {
			if err != io.EOF {
				// Handle the error
			}

			break
		}
		select {
		case v.OpusSend <- frame:
		case <-time.After(time.Second):
			// We haven't been able to send a frame in a second, assume the connection is borked
			return
		}
	}
}
