package main

import (
	"bufio"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"runtime/debug"
	"time"

	"github.com/bwmarrin/discordgo"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/websocket"
	"github.com/jonas747/dca"
)

func Recover() {
	if r := recover(); r != nil {
		debug.PrintStack()
	}
}

func ReturnError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	w.Write([]byte(msg))
}

func StartRange(guildID, channelID string) {
	defer Recover()
	defer delete(voiceConnection, guildID)

	voiceConnection[guildID].QueueStatus = true

	for voiceConnection[guildID].QueueStatus {
		videoQueue := GetVideoQueue(guildID)

		if len(videoQueue) == 0 {
			continue
		}

		log.Printf("재생: %s", videoQueue[0].Title)
		discord.ChannelMessageSend(channelID, fmt.Sprintf("🎶 `재생: %s`", videoQueue[0].Title))

		TTSAction(videoQueue[0])

		log.Printf("종료: %s", videoQueue[0].Title)
		if err := RemoveQueue(guildID, videoQueue[0].QueueID); err != nil {
			continue
		}
	}
}

func GetVideoQueue(guildID string) []*VideoQueue {
	defer Recover()

	database, _ := sql.Open("mysql", mysqlServer)
	defer database.Close()

	rows, err := database.Query("SELECT id, video_id, video_title, video_channel, video_duration, video_thumbnail FROM queue WHERE guild_id = ?", guildID)
	if err != nil {
		fmt.Println(err)
	}
	defer rows.Close()

	var videoQueue []*VideoQueue
	var videoID, videoTitle, videoChannel, videoThumbnail string
	var id, videoDuration int

	for rows.Next() {
		rows.Scan(&id, &videoID, &videoTitle, &videoChannel, &videoDuration, &videoThumbnail)

		videoQueue = append(videoQueue, &VideoQueue{
			QueueID:   id,
			GuildID:   guildID,
			ID:        videoID,
			Title:     videoTitle,
			Channel:   videoChannel,
			Duration:  videoDuration,
			Thumbnail: videoThumbnail,
		})
	}

	return videoQueue
}

func AddQueue(videoQueue *VideoQueue) error {
	defer Recover()

	database, err := sql.Open("mysql", mysqlServer)
	if err != nil {
		fmt.Println(err)
	}
	defer database.Close()

	_, err = database.Exec("INSERT INTO queue(guild_id, video_id, video_title, video_channel, video_duration, video_thumbnail) VALUES(?, ?, ?, ?, ?, ?)", videoQueue.GuildID, videoQueue.ID, videoQueue.Title, videoQueue.Channel, videoQueue.Duration, videoQueue.Thumbnail)
	if err != nil {
		fmt.Println(err)
	}

	if err := SendWebsocket(websocket.TextMessage, videoQueue.GuildID, 0, map[string]interface{}{
		"type":             "update_queue",
		"video_queue_info": GetVideoQueue(videoQueue.GuildID),
	}); err != nil {
		return err
	}

	return nil
}

func GetYoutubeMusicDuration(id string) (time.Duration, error) {
	defer Recover()

	ytVideo, err := yt.GetVideo("https://www.youtube.com/watch?v=" + id)
	if err != nil {
		return 0, err
	}

	return ytVideo.Duration, nil
}

func GetYoutubeMusic(id string) (*http.Response, error) {
	defer Recover()

	ytVideo, err := yt.GetVideo("https://www.youtube.com/watch?v=" + id)
	if err != nil {
		log.Println("Get Video 에러")

		return nil, err
	}

	resp, err := yt.GetStream(ytVideo, ytVideo.Formats.FindByItag(140))
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}

		log.Println("Get Stream 에러")

		return nil, err
	}

	return resp, nil
}

func GetYoutubeSearchList(q string) (YoutubeSearch, error) {
	defer Recover()

	apiURL := "https://www.googleapis.com/youtube/v3/search"
	apiURL += "?key=" + youtubeAPIKey
	apiURL += "&part=snippet&type=video&maxResults=20&videoEmbeddable=true"
	apiURL += "&q=" + url.QueryEscape(q)

	resp, err := http.Get(apiURL)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}

		return YoutubeSearch{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return YoutubeSearch{}, err
	}

	var youtubeSearch YoutubeSearch
	json.Unmarshal(body, &youtubeSearch)

	return youtubeSearch, nil
}

func GetYoutubeRelatedList(id string) ([]YoutubeSearch, error) {
	defer Recover()

	var list []YoutubeSearch
	var pageToken string

	for i := 0; i < 3; i++ {
		apiURL := "https://www.googleapis.com/youtube/v3/search"
		apiURL += "?key=" + youtubeAPIKey
		apiURL += "&part=snippet&type=video&maxResults=20&videoEmbeddable=true"
		apiURL += "&pageToken=" + pageToken
		apiURL += "&relatedToVideoId=" + id

		resp, err := http.Get(apiURL)
		if err != nil {
			if resp != nil {
				resp.Body.Close()
			}

			break
		}

		body, err := ioutil.ReadAll(resp.Body)
		if resp != nil {
			resp.Body.Close()
		}

		if err != nil {
			return nil, err
		}

		var youtubeSearch YoutubeSearch
		json.Unmarshal(body, &youtubeSearch)

		list = append(list, youtubeSearch)
	}

	return list, nil
}

func GetYoutubeVideoInfo(id string) (YoutubeSearch, error) {
	defer Recover()

	apiURL := "https://www.googleapis.com/youtube/v3/videos"
	apiURL += "?key=" + youtubeAPIKey
	apiURL += "&part=snippet"
	apiURL += "&id=" + id

	resp, err := http.Get(apiURL)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}

		return YoutubeSearch{}, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return YoutubeSearch{}, err
	}

	var youtubeSearch YoutubeSearch
	json.Unmarshal(body, &youtubeSearch)

	return youtubeSearch, nil
}

func SendErrorMessage(s *discordgo.Session, channelID string, code int) {
	defer Recover()

	s.ChannelMessageSend(channelID, fmt.Sprintf("```cs\n"+
		"# 에러가 발생했습니다. 잠시 후 다시 사용해주세요.\n"+
		"CODE: %d\n"+
		"```",
		code,
	))
}

func RemoveQueue(guildID string, queueID int) error {
	defer Recover()

	database, _ := sql.Open("mysql", mysqlServer)
	defer database.Close()

	_, _ = database.Exec("DELETE FROM queue WHERE id = ?", queueID)

	if err := SendWebsocket(websocket.TextMessage, guildID, 0, map[string]interface{}{
		"type":             "update_queue",
		"video_queue_info": GetVideoQueue(guildID),
	}); err != nil {
		return err
	}

	return nil
}

func VerifyUser(uniq string) (ok bool, userID string) {
	database, _ := sql.Open("mysql", mysqlServer)
	defer database.Close()

	var uniqDB string

	_ = database.QueryRow("SELECT user_id, uniq FROM user").Scan(&userID, &uniqDB)

	if uniq == uniqDB {
		return true, userID
	}

	return false, userID
}

func EncryptUniq(guildID, ip string) string {
	hash := sha256.New()
	hash.Write([]byte(guildID + ip))

	return hex.EncodeToString(hash.Sum(nil))
}

func GetAccessToken(uniq string) string {
	hash := sha256.New()
	hash.Write([]byte(fmt.Sprintf("%s%d", uniq, time.Now().Day())))

	return hex.EncodeToString(hash.Sum(nil))
}

func GetBeforeAccessToken(uniq string) string {
	hash := sha256.New()
	hash.Write([]byte(fmt.Sprintf("%s%d", uniq, time.Now().AddDate(0, 0, -1).Day())))

	return hex.EncodeToString(hash.Sum(nil))
}

func AddVerifyUser(guildID, ip, code string) bool {
	if voiceConnection[guildID] == nil || len(voiceConnection[guildID].ChannelID) == 0 {
		return false
	}

	database, _ := sql.Open("mysql", mysqlServer)
	defer database.Close()

	if _, ok := userVerificationCode[guildID]; !ok {
		userVerificationCode[guildID] = make(map[string]*UserInfo)
	}

	userVerificationCode[guildID][code] = &UserInfo{
		IP: ip,
	}
	defer delete(userVerification[guildID], code)

	userVerification[guildID] = make(map[string]chan *UserInfo)
	userVerification[guildID][ip] = make(chan *UserInfo)
	defer close(userVerification[guildID][ip])

	nowTime := time.Now()
	ticker := time.NewTicker(time.Millisecond)

	log.Printf("[%s][%s] 코드: %s 기다리는 중...", guildID, ip, code)
	for {
		select {
		case result := <-userVerification[guildID][ip]:
			log.Println("승인")
			_, _ = database.Exec("INSERT INTO user(user_id, uniq) VALUES(?, ?)", result.UserID, EncryptUniq(guildID, ip))

			return true
		case <-ticker.C:
			if time.Since(nowTime).Minutes() > 3 {
				log.Println("비승인")

				return false
			}
		}
	}
}

func TTSAction(item *VideoQueue) {
	defer Recover()

	if err := voiceConnection[item.GuildID].VC.Speaking(true); err != nil {
		fmt.Println(err)
	}

	resp, err := GetYoutubeMusic(item.ID)
	if err != nil {
		if resp != nil {
			resp.Body.Close()
		}

		fmt.Println(err)
	}

	var encodingSession *dca.EncodeSession

	websocketTicker := time.NewTicker(100 * time.Millisecond)

	voiceConnection[item.GuildID].Done = make(chan error)

	options.BufferedFrames = int(resp.ContentLength)
	options.StartTime = 0

	encodingSession, _ = dca.EncodeMem(bufio.NewReaderSize(resp.Body, int(resp.ContentLength)), options)
	voiceConnection[item.GuildID].StreamSession = dca.NewStream(encodingSession, voiceConnection[item.GuildID].VC, voiceConnection[item.GuildID].Done)

	defer func() {
		log.Printf("초기화 중...")
		voiceConnection[item.GuildID].VC.Speaking(false)
		websocketTicker.Stop()
		resp.Body.Close()
		encodingSession.Stop()
		encodingSession.Cleanup()
	}()

	for {
		select {
		case err := <-voiceConnection[item.GuildID].Done:
			if err != io.EOF {
				return
			}

			return
		case <-websocketTicker.C:
			_ = SendWebsocket(websocket.TextMessage, item.GuildID, 0, map[string]interface{}{
				"type":              "update_time",
				"playback_position": voiceConnection[item.GuildID].StreamSession.PlaybackPosition().Seconds() + float64(options.StartTime),
				"duration":          item.Duration,
			})
		case startTime := <-voiceConnection[item.GuildID].StartTime:
			resp.Body.Close()

			resp, _ = GetYoutubeMusic(item.ID)

			encodingSession.Stop()
			encodingSession.Cleanup()

			voiceConnection[item.GuildID].Done = make(chan error)

			options.StartTime = startTime

			encodingSession, _ = dca.EncodeMem(bufio.NewReaderSize(resp.Body, int(resp.ContentLength)), options)
			voiceConnection[item.GuildID].StreamSession = dca.NewStream(encodingSession, voiceConnection[item.GuildID].VC, voiceConnection[item.GuildID].Done)

			for {
				if voiceConnection[item.GuildID].StreamSession.PlaybackPosition().Seconds() > 0 {
					_ = SendWebsocket(websocket.TextMessage, item.GuildID, 0, map[string]interface{}{
						"type": "loading_done",
					})

					break
				}
			}
		}
	}
}

func SendWebsocket(messageType int, guildID string, clientID int64, data map[string]interface{}) error {
	defer Recover()

	sendData, _ := json.Marshal(data)

	if clientID == 0 {
		for _, item := range conn[guildID] {
			if item == nil {
				continue
			}

			if err := item.Conn.WriteMessage(messageType, sendData); err != nil {
				continue
			}
		}
	} else {
		if _, ok := conn[guildID]; !ok {
			return errors.New("error")
		}

		if conn[guildID][clientID] == nil {
			return errors.New("error")
		}

		if err := conn[guildID][clientID].Conn.WriteMessage(messageType, sendData); err != nil {
			return err
		}
	}

	return nil
}

func TTSSkip(guildID string) {
	defer Recover()

	log.Println("스킵: " + guildID)
	voiceConnection[guildID].Done <- io.EOF
}

func LeaveChannel(guildID string) {
	defer Recover()

	log.Println("퇴장: " + guildID)
	TTSSkip(guildID)
	voiceConnection[guildID].QueueStatus = false
	_ = voiceConnection[guildID].VC.Disconnect()
	voiceConnection[guildID].VC.Close()
}

func FindUserVoiceState(session *discordgo.Session, userid string) (*discordgo.VoiceState, error) {
	defer Recover()

	for _, guild := range session.State.Guilds {
		for _, vs := range guild.VoiceStates {
			if vs.UserID == userid {
				return vs, nil
			}
		}
	}

	return nil, errors.New("error")
}

func JoinVoiceChannel(channelID string) (*discordgo.VoiceConnection, error) {
	defer Recover()

	channel, err := discord.Channel(channelID)
	if err != nil {
		return nil, err
	}

	vc, err := discord.ChannelVoiceJoin(channel.GuildID, channelID, false, false)
	if err != nil {
		if _, ok := discord.VoiceConnections[channel.GuildID]; ok {
			vc = discord.VoiceConnections[channel.GuildID]
		} else {
			return nil, err
		}
	}

	return vc, nil
}

func GetWord(word, userid string) string {
	defer Recover()

	database, _ := sql.Open("mysql", mysqlServer)
	defer database.Close()

	rows, err := database.Query(fmt.Sprintf("SELECT item FROM word WHERE item LIKE '%s%%'", word))
	if err != nil {
		fmt.Println(err)
	}
	defer rows.Close()

	var itemDB string
	var item []string

	for rows.Next() {
		rows.Scan(&itemDB)

		if len([]rune(itemDB)) == 1 {
			continue
		}

		if !CheckExist(itemDB, userid) {
			item = append(item, itemDB)
		}
	}

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(item), func(i, j int) {
		item[i], item[j] = item[j], item[i]
	})

	return item[0]
}

func CheckWord(word string) bool {
	defer Recover()

	database, _ := sql.Open("mysql", mysqlServer)
	defer database.Close()

	rows, err := database.Query("SELECT item FROM word")
	if err != nil {
		fmt.Println(err)
	}
	defer rows.Close()

	var itemDB string
	for rows.Next() {
		rows.Scan(&itemDB)

		if itemDB == word {
			return true
		}
	}

	return false
}

func CheckExist(word, userid string) bool {
	defer Recover()

	for _, item := range users[userid].WordLog {
		if item == word {
			return true
		}
	}

	return false
}
