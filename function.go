package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jonas747/dca"
	ytClient "github.com/kkdai/youtube/v2"
	_ "github.com/mattn/go-sqlite3"
)

func GetYoutubeMusic(id string) (*http.Response, time.Duration, error) {
	yt := ytClient.Client{}
	ytVideo, err := yt.GetVideo("https://www.youtube.com/watch?v=" + id)
	if err != nil {
		return nil, 0, err
	}

	resp, err := yt.GetStream(ytVideo, ytVideo.Formats.FindByItag(140))
	if err != nil {
		return nil, 0, err
	}

	return resp, ytVideo.Duration, nil
}

func GetYoutubeSearchList(q string) (YoutubeSearch, error) {
	apiURL := "https://www.googleapis.com/youtube/v3/search"
	apiURL += "?key=" + YoutubeAPIKey
	apiURL += "&part=snippet&type=video&maxResults=20&videoEmbeddable=true"
	apiURL += "&q=" + url.QueryEscape(q)

	resp, err := http.Get(apiURL)
	if err != nil {
		return YoutubeSearch{}, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return YoutubeSearch{}, err
	}

	var youtubeSearch YoutubeSearch
	json.Unmarshal(body, &youtubeSearch)

	return youtubeSearch, nil
}

func SendErrorMessage(s *discordgo.Session, channelID string, code int) {
	s.ChannelMessageSend(channelID, fmt.Sprintf("```cs\n"+
		"# 에러가 발생했습니다. 잠시 후 다시 사용해주세요.\n"+
		"CODE: %d\n"+
		"```",
		code,
	))
}

func RemoveQueue(guildID string, unixNano int64) []*VideoQueueInfo {
	var s []*VideoQueueInfo

	for _, item := range videoQueueInfo[guildID] {
		if unixNano == item.UnixNano {
			continue
		}

		s = append(s, item)
	}

	return s
}

func TTSActionFromFile(vc *discordgo.VoiceConnection, path string) {
	encodingSession, _ := dca.EncodeFile(path, dca.StdEncodeOptions)

	done := make(chan error)
	dca.NewStream(encodingSession, vc, done)
	err := <-done
	if err != nil && err != io.EOF {
		fmt.Println(err)
	}

	encodingSession.Cleanup()
}

func TTSAction(vc *VoiceConnection, item *VideoQueue) {
	options := dca.StdEncodeOptions
	options.BufferedFrames = item.BufferLength
	options.RawOutput = true
	options.Bitrate = 96
	options.Application = "lowdelay"

	encodingSession, _ := dca.EncodeMem(item.Reader, options)
	voiceConnection[vc.GuildID].EncodingSession = encodingSession

	dca.NewStream(encodingSession, vc.VC, vc.Done)
	err := <-vc.Done
	if err != nil && err != io.EOF {
		fmt.Println(err)
	}

	err = encodingSession.Stop()
	if err != nil {
		fmt.Println(err)
	}
	encodingSession.Cleanup()
}

func TTSSkip(guildID string) {
	err := voiceConnection[guildID].EncodingSession.Stop()
	if err != nil {
		fmt.Println(err)
	}
	voiceConnection[guildID].EncodingSession.Cleanup()
}

func FindUserVoiceState(session *discordgo.Session, userid string) (*discordgo.VoiceState, error) {
	for _, guild := range session.State.Guilds {
		for _, vs := range guild.VoiceStates {
			if vs.UserID == userid {
				return vs, nil
			}
		}
	}

	return nil, errors.New("error")
}

func JoinVoiceChannel(session *discordgo.Session, channelID string) (*discordgo.VoiceConnection, error) {
	channel, err := session.Channel(channelID)
	if err != nil {
		return nil, err
	}

	return session.ChannelVoiceJoin(channel.GuildID, channelID, false, false)
}

func GetWord(word, userid string) (*sql.DB, *sql.Rows, bool, string) {
	database, _ := sql.Open("sqlite3", "./word.db")

	rows, err := database.Query(fmt.Sprintf("SELECT item FROM word WHERE item LIKE '%s%%'", word))
	if err != nil {
		fmt.Println(err)
	}

	var itemDB string
	for rows.Next() {
		rows.Scan(&itemDB)

		if len([]rune(itemDB)) == 1 {
			continue
		}

		if !CheckExist(itemDB, userid) {
			return database, rows, true, itemDB
		}
	}

	return database, rows, false, ""
}

func CheckWord(word string) (*sql.DB, *sql.Rows, bool) {
	database, _ := sql.Open("sqlite3", "./word.db")

	rows, err := database.Query("SELECT item FROM word")
	if err != nil {
		fmt.Println(err)
	}

	var itemDB string
	for rows.Next() {
		rows.Scan(&itemDB)

		if itemDB == word {
			return database, rows, true
		}
	}

	return database, rows, false
}

func CheckExist(word, userid string) bool {
	for _, item := range users[userid].WordLog {
		if item == word {
			return true
		}
	}

	return false
}
