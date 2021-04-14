package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/jonas747/dca"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/oggwriter"
)

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
						go handleVoiceChannel(v, c, s, m, guild.ID)
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
	}
}

func voiceSpeakingUpdate(vc *discordgo.VoiceConnection, vs *discordgo.VoiceSpeakingUpdate) {
	fmt.Println("Someone is speaking")
	// map who is speaking to a global state.
	// at some point, entries in this map should be garbage collected. Likely when we leave a channel
	SpeakerStreams[vs.SSRC] = vs.UserID
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

func handleVoiceChannel(v *discordgo.VoiceConnection, c chan struct{}, s *discordgo.Session, m *discordgo.MessageCreate, gid string) {

	//play a sound when we join
	// v.Speaking(true)
	// playSound(v)
	// v.Speaking(false)

	// attach handler to channel for speaking updates
	v.AddHandler(voiceSpeakingUpdate)

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintln("Started recording..."))
	//background function that listens for the leave message to close up shop
	go func() {
		<-c
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintln("Leaving & stopping recording..."))
		//this breaks the for loop below
		close(v.OpusRecv)
	}()

	// get date for folder
	d := time.Now().Unix()
	err := os.MkdirAll(fmt.Sprintf("recordings/%d", d), 0755)
	if err != nil {
		panic(err)
	}
	files := make(map[uint32]media.Writer)
	for p := range v.OpusRecv {
		file, ok := files[p.SSRC]
		if !ok {
			// look up user for this stream. Block until the map has it cause there's a potential race. Any block should hopefully queue up on the other end and not affect the audio stream
			found := false
			for !found {
				if _, ok2 := SpeakerStreams[int(p.SSRC)]; ok2 {
					found = true
				} else {
					// i'm polling for now
					time.Sleep(50 * time.Millisecond)
				}
			}
			user, err := s.User(SpeakerStreams[int(p.SSRC)])
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintln(err))
			}
			// we're ready to open the file
			file, err = oggwriter.New(fmt.Sprintf("recordings/%d/%s-%d.ogg", d, user.Username, time.Now().Unix()), 48000, 2)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("failed to create file recordings/%d/%s-%d.ogg, giving up on recording: %v\n", d, user.Username, time.Now().Unix(), err))
			}
			files[p.SSRC] = file
		}
		// Construct pion RTP packet from DiscordGo's type.
		rtp := createPionRTPPacket(p)
		err = file.WriteRTP(rtp)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("failed to write to file. giving up on recording: %v\n", err))
		}
	}
	// Once we made it here, we're done listening for packets. Close the files
	for key, f := range files {
		// Remove SSRC entries from the speaker mapping (avoid a memory leak)
		delete(SpeakerStreams, int(key))
		// Close the file
		f.Close()
	}
	// Close the voice web socket
	v.Close()
	// Remove ourselves from the global mapping
	delete(VoiceConnections, gid)

	// Disconnect the bot
	v.Disconnect()

	//find all the ogg files
	ogg_files, err := filepath.Glob(fmt.Sprintf("recordings/%d/*.ogg", d))
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Can't zip up ogg files: %v\n", err))
	}
	// zip up what we found
	if err := ZipFiles(fmt.Sprintf("recordings/%d.zip", d), ogg_files); err != nil {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Error when zipping ogg files: %v\n", err))
	}
	// generate a uuid for this session
	id := uuid.New().String()
	// store it in the mapping table
	ZipMap[id] = fmt.Sprintf("recordings/%d.zip", d)
	//flush it to disk
	storeZipMap()
	// tell the user how to get their recording
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("The zip for this session lives at %s/%s.zip", Hostname, id))
}

func playSound(v *discordgo.VoiceConnection) {
	// Encoding a file and saving it to disk
	encodeSession, err := dca.EncodeFile("welcome.ogg", dca.StdEncodeOptions)
	if err != nil {
		fmt.Println(err)
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
