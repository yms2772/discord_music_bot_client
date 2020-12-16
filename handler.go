package main

import (
	"fmt"
	"html"
	"log"
	url2 "net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func OnUpdateStatus(s *discordgo.Session, _ *discordgo.Ready) {
	defer Recover()

	_ = s.UpdateListeningStatus("음악")
}

func OnMessageUpdate(s *discordgo.Session, m *discordgo.MessageCreate) {
	defer Recover()

	OnWordChainMessage(s, m.Message)
	OnMusicMessage(s, m.Message)
}

func OnMusicMessage(s *discordgo.Session, m *discordgo.Message) {
	defer Recover()

	if s.State.Ready.User.Username == m.Author.Username {
		return
	}

	method := strings.Split(m.Content, " ")

	if len(method) < 1 {
		return
	}

	vcState, err := FindUserVoiceState(s, m.Author.ID)
	if err != nil {
		fmt.Println(err)

		return
	}

	switch method[0] {
	case "~p", "~pl", "~pr", "~pn":
		if method[0] != "~pn" && len(method) < 2 {
			s.ChannelMessageSend(m.ChannelID, "```cs\n"+
				"# 사용법: {~p, ~pl} 제목\n"+
				"```",
			)

			return
		}

		if _, ok := voiceConnection[m.GuildID]; !ok || !voiceConnection[m.GuildID].VC.Ready {
			log.Printf("연결: %s", m.GuildID)
			channel, _ := s.Channel(vcState.ChannelID)
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🔗 `연결: %s`", channel.Name))

			vc, err := JoinVoiceChannel(vcState.ChannelID)
			if err != nil {
				if vc != nil {
					vc.Close()
				}

				fmt.Println(err)
				SendErrorMessage(s, m.ChannelID, 10000)

				return
			}

			voiceConnection[m.GuildID] = &VoiceConnection{
				VoiceOption: VoiceOption{
					Volume: 256,
				},
				GuildID:   m.GuildID,
				ChannelID: m.ChannelID,
				VC:        vc,
				StartTime: make(chan int),
			}

			if !voiceConnection[m.GuildID].QueueStatus {
				go StartRange(m.GuildID, m.ChannelID)
			}
		}

		if method[0] == "~pn" {
			return
		}

		q := strings.Join(method[1:], " ")

		log.Printf("%s 검색 중...", q)
		var videoDuration time.Duration
		var list YoutubeSearch
		var relatedList []YoutubeSearch
		var videoID, videoTitle, videoThumbnail, videoChannel string
		var videoDurationSeconds int
		var totalVideo, currentVideo, cantPlay int

		searching, _ := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🎵 `검색 중: %s`", q))

		urlParse, err := url2.Parse(q)
		if err != nil {
			_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```cs\n"+
				"# 검색결과가 없습니다.\n"+
				"```",
			))

			return
		}

		switch urlParse.Host {
		case "youtube.com", "www.youtube.com":
			log.Println("Full URL 검색")
			urlQuery, _ := url2.ParseQuery(urlParse.RawQuery)
			id := urlQuery["v"]

			if len(id) == 0 {
				_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)
				SendErrorMessage(s, m.ChannelID, 10011)

				return
			}

			list, err = GetYoutubeVideoInfo(id[0])
			if err != nil {
				_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)
				SendErrorMessage(s, m.ChannelID, 10021)

				return
			}

			videoID = id[0]
			log.Printf("Video ID: %s", videoID)
		case "youtu.be", "www.youtu.be":
			log.Println("단축 URL 검색")
			list, err = GetYoutubeVideoInfo(urlParse.Path[1:])
			if err != nil {
				_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)
				SendErrorMessage(s, m.ChannelID, 10031)

				return
			}

			videoID = urlParse.Path[1:]
			log.Printf("Video ID: %s", videoID)
		default:
			list, err = GetYoutubeSearchList(q)
			if err != nil {
				_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)
				SendErrorMessage(s, m.ChannelID, 10001)

				return
			}
		}

		if len(list.Items) < 1 {
			log.Println("Item Size: 0")
			_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```cs\n"+
				"# 검색결과가 없습니다.\n"+
				"```",
			))

			return
		}

		errorCount := 0
		result := list.Items[errorCount]

		if len(videoID) == 0 {
			videoDuration, err = GetYoutubeMusicDuration(result.ID.VideoID)
			for err != nil {
				errorCount++

				fmt.Println(err)

				if errorCount > 5 {
					_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)
					SendErrorMessage(s, m.ChannelID, 10002)

					return
				}

				if len(list.Items) == errorCount {
					_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)
					s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```cs\n"+
						"# 검색결과가 없습니다.\n"+
						"```",
					))

					return
				}

				result = list.Items[errorCount]
				videoDuration, err = GetYoutubeMusicDuration(result.ID.VideoID)
			}

			videoID = result.ID.VideoID
			videoTitle = html.UnescapeString(result.Snippet.Title)
			videoThumbnail = result.Snippet.Thumbnails.High.URL
			videoChannel = html.UnescapeString(result.Snippet.ChannelTitle)
			videoDurationSeconds = int(videoDuration.Seconds())
		} else {
			videoDuration, err = GetYoutubeMusicDuration(videoID)
			if err != nil {
				fmt.Println(err)

				_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```cs\n"+
					"# 검색결과가 없습니다.\n"+
					"```",
				))

				return
			}

			videoTitle = html.UnescapeString(result.Snippet.Title)
			videoThumbnail = result.Snippet.Thumbnails.High.URL
			videoChannel = html.UnescapeString(result.Snippet.ChannelTitle)
			videoDurationSeconds = int(videoDuration.Seconds())
		}

		videoDurationH := videoDurationSeconds / 3600
		videoDurationM := (videoDurationSeconds - (3600 * videoDurationH)) / 60
		videoDurationS := videoDurationSeconds - (3600 * videoDurationH) - (videoDurationM * 60)

		switch method[0] {
		case "~pl":
			voiceConnection[m.GuildID].StopRelatedVideo = false

			log.Printf("연관된 영상 목록 가져오는 중...")
			relatedList, err = GetYoutubeRelatedList(result.ID.VideoID)
			if err != nil {
				_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)
				SendErrorMessage(s, m.ChannelID, 20001)

				return
			}

			if len(relatedList) < 1 {
				_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```cs\n"+
					"# 검색결과가 없습니다.\n"+
					"```",
				))

				return
			}

			for _, page := range relatedList {
				for range page.Items {
					totalVideo++
				}
			}

			log.Printf("검색된 연관된 영상 수: %d", totalVideo)
		}

		log.Printf("검색된 영상: %s (%s) (%d초)", videoTitle, result.ID.VideoID, videoDurationSeconds)
		_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)

		if len(GetVideoQueue(m.GuildID)) != 0 {
			s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
				Author: &discordgo.MessageEmbedAuthor{
					URL:     "http://toy.mokky.kr/server/" + m.GuildID,
					Name:    "재생목록 추가",
					IconURL: m.Author.AvatarURL(""),
				},
				Footer: &discordgo.MessageEmbedFooter{
					Text:    "Youtube",
					IconURL: "https://toy.mokky.kr/web/favicon/youtube.png",
				},
				Color:       Blue,
				Description: fmt.Sprintf("[%s](%s)", videoTitle, "https://www.youtube.com/watch?v="+videoID),
				Thumbnail: &discordgo.MessageEmbedThumbnail{
					URL: videoThumbnail,
				},
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:   "채널",
						Value:  videoChannel,
						Inline: true,
					},
					{
						Name:   "영상 시간",
						Value:  fmt.Sprintf("%02d:%02d:%02d", videoDurationH, videoDurationM, videoDurationS),
						Inline: true,
					},
					{
						Name:   "대기열",
						Value:  fmt.Sprintf("%d", len(GetVideoQueue(m.GuildID))),
						Inline: true,
					},
				},
			})
		}

		log.Println("대기열 전송 중...")
		_ = AddQueue(&VideoQueue{
			GuildID:   m.GuildID,
			ID:        videoID,
			Channel:   videoChannel,
			Title:     videoTitle,
			Duration:  videoDurationSeconds,
			Thumbnail: videoThumbnail,
		})

		switch method[0] {
		case "~pl", "~playlist":
		LIST:
			for _, page := range relatedList {
			ITEM:
				for _, item := range page.Items {
					if _, ok := voiceConnection[m.GuildID]; !ok || voiceConnection[m.GuildID].StopRelatedVideo {
						s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ `종료: %s`", q))

						break LIST
					}

					errorCount = 0

					videoDuration, err := GetYoutubeMusicDuration(item.ID.VideoID)
					for err != nil {
						errorCount++

						if errorCount > 10 {
							cantPlay++
							totalVideo--

							continue ITEM
						}

						videoDuration, err = GetYoutubeMusicDuration(item.ID.VideoID)
					}

					videoID := item.ID.VideoID
					videoTitle := html.UnescapeString(item.Snippet.Title)
					videoThumbnail := item.Snippet.Thumbnails.High.URL
					videoChannel := html.UnescapeString(item.Snippet.ChannelTitle)
					videoDurationSeconds := int(videoDuration.Seconds())

					videoDurationH := videoDurationSeconds / 3600
					videoDurationM := (videoDurationSeconds - (3600 * videoDurationH)) / 60
					videoDurationS := videoDurationSeconds - (3600 * videoDurationH) - (videoDurationM * 60)

					log.Printf("검색된 영상: %s (%s) (%d초)", videoTitle, item.ID.VideoID, videoDurationSeconds)
					_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)

					s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
						Author: &discordgo.MessageEmbedAuthor{
							URL:     "http://toy.mokky.kr/server/" + m.GuildID,
							Name:    "재생목록 추가",
							IconURL: m.Author.AvatarURL(""),
						},
						Footer: &discordgo.MessageEmbedFooter{
							Text:    "Youtube",
							IconURL: "https://toy.mokky.kr/web/favicon/youtube.png",
						},
						Color:       Blue,
						Description: fmt.Sprintf("[%s](%s)", videoTitle, "https://www.youtube.com/watch?v="+videoID),
						Thumbnail: &discordgo.MessageEmbedThumbnail{
							URL: videoThumbnail,
						},
						Fields: []*discordgo.MessageEmbedField{
							{
								Name:   "채널",
								Value:  videoChannel,
								Inline: true,
							},
							{
								Name:   "영상 시간",
								Value:  fmt.Sprintf("%02d:%02d:%02d", videoDurationH, videoDurationM, videoDurationS),
								Inline: true,
							},
							{
								Name:   "대기열",
								Value:  fmt.Sprintf("%d/%d", currentVideo+1, totalVideo),
								Inline: true,
							},
						},
					})

					log.Println("대기열 전송 중...")
					currentVideo++
					_ = AddQueue(&VideoQueue{
						GuildID:   m.GuildID,
						ID:        videoID,
						Channel:   videoChannel,
						Title:     videoTitle,
						Duration:  videoDurationSeconds,
						Thumbnail: videoThumbnail,
					})

					for len(GetVideoQueue(m.GuildID)) > 5 {
						time.Sleep(time.Second)
					}
				}
			}
		case "~pr":
			voiceConnection[m.GuildID].StopRelatedVideo = false

			for {
				if _, ok := voiceConnection[m.GuildID]; !ok || voiceConnection[m.GuildID].StopRelatedVideo {
					s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ `종료: %s`", q))

					break
				}

				log.Println("대기열 전송 중...")
				currentVideo++
				_ = AddQueue(&VideoQueue{
					GuildID:   m.GuildID,
					ID:        videoID,
					Channel:   videoChannel,
					Title:     videoTitle,
					Duration:  videoDurationSeconds,
					Thumbnail: videoThumbnail,
				})

				for len(GetVideoQueue(m.GuildID)) > 2 {
					time.Sleep(time.Second)
				}
			}
		}

		log.Println("대기열 전송 완료")
	case "~c", "~cancel":
		voiceConnection[m.GuildID].StopRelatedVideo = true

		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```md\n"+
			"# 다음 곡 까지만 재생됩니다\n"+
			"```",
		))
	case "~np":
		videoQueue := GetVideoQueue(m.GuildID)
		if len(videoQueue) == 0 {
			return
		}

		guild, _ := s.Guild(m.GuildID)

		videoDurationSeconds := videoQueue[0].Duration
		videoCurrentSeconds := int(voiceConnection[m.GuildID].StreamSession.PlaybackPosition().Seconds())
		videoRemainSeconds := videoDurationSeconds - videoCurrentSeconds
		videoControlBarPoint := int((float64(videoCurrentSeconds) / float64(videoDurationSeconds)) * 10)

		videoCurrentH := videoCurrentSeconds / 3600
		videoCurrentM := (videoCurrentSeconds - (3600 * videoCurrentH)) / 60
		videoCurrentS := videoCurrentSeconds - (3600 * videoCurrentH) - (videoCurrentM * 60)

		videoDurationH := videoDurationSeconds / 3600
		videoDurationM := (videoDurationSeconds - (3600 * videoDurationH)) / 60
		videoDurationS := videoDurationSeconds - (3600 * videoDurationH) - (videoDurationM * 60)

		videoRemainH := videoRemainSeconds / 3600
		videoRemainM := (videoRemainSeconds - (3600 * videoRemainH)) / 60
		videoRemainS := videoRemainSeconds - (3600 * videoRemainH) - (videoRemainM * 60)

		var videoControlBar string

		for i := 0; i < 10; i++ {
			if i == videoControlBarPoint {
				videoControlBar += "♩"

				continue
			}

			videoControlBar += "━"
		}

		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Author: &discordgo.MessageEmbedAuthor{
				URL:     "http://toy.mokky.kr/server/" + m.GuildID,
				Name:    "현재 재생 중",
				IconURL: guild.IconURL(),
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text:    "Youtube",
				IconURL: "https://toy.mokky.kr/web/favicon/youtube.png",
			},
			Color: Blue,
			Description: fmt.Sprintf("[%s](%s)\n"+
				"`%02d:%02d:%02d` |%s| `%02d:%02d:%02d`",
				videoQueue[0].Title, "https://www.youtube.com/watch?v="+videoQueue[0].ID,
				videoCurrentH, videoCurrentM, videoCurrentS, videoControlBar, videoDurationH, videoDurationM, videoDurationS,
			),
			Thumbnail: &discordgo.MessageEmbedThumbnail{
				URL: videoQueue[0].Thumbnail,
			},
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "채널",
					Value:  videoQueue[0].Channel,
					Inline: true,
				},
				{
					Name:   "영상 시간",
					Value:  fmt.Sprintf("%02d:%02d:%02d", videoDurationH, videoDurationM, videoDurationS),
					Inline: true,
				},
				{
					Name:   "남은 시간",
					Value:  fmt.Sprintf("%02d:%02d:%02d", videoRemainH, videoRemainM, videoRemainS),
					Inline: true,
				},
			},
		})
	case "~q":
		var data string
		var videoRemainSeconds int

		videoQueue := GetVideoQueue(m.GuildID)
		guild, _ := s.Guild(m.GuildID)

		for i, item := range videoQueue {
			if i == 0 {
				data += fmt.Sprintf("%d. [%s](%s)\n", i+1, item.Title, "https://www.youtube.com/watch?v="+item.ID)
			} else {
				if i-1 == 0 {
					videoRemainSeconds += videoQueue[i-1].Duration - int(voiceConnection[m.GuildID].StreamSession.PlaybackPosition().Seconds())
				} else {
					videoRemainSeconds += videoQueue[i-1].Duration
				}

				videoRemainH := videoRemainSeconds / 3600
				videoRemainM := (videoRemainSeconds - (3600 * videoRemainH)) / 60
				videoRemainS := videoRemainSeconds - (3600 * videoRemainH) - (videoRemainM * 60)

				data += fmt.Sprintf("%d. [%s](%s) `%02d:%02d:%02d 남음`\n", i+1, item.Title, "https://www.youtube.com/watch?v="+item.ID, videoRemainH, videoRemainM, videoRemainS)
			}
		}

		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Author: &discordgo.MessageEmbedAuthor{
				URL:     "http://toy.mokky.kr/server/" + m.GuildID,
				Name:    fmt.Sprintf("%s의 재생목록", guild.Name),
				IconURL: guild.IconURL(),
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text:    "Youtube",
				IconURL: "https://toy.mokky.kr/web/favicon/youtube.png",
			},
			Color:       Pink,
			Description: data,
		})
	case "~fs", "~force_skip":
		if item := GetVideoQueue(m.GuildID); len(item) != 0 {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("⏭ `건너뛰기: %s`", item[0].Title))

			TTSSkip(m.GuildID)
		}
	case "~l", "~leave":
		if _, ok := voiceConnection[m.GuildID]; !ok || !voiceConnection[m.GuildID].VC.Ready {
			s.ChannelMessageSend(m.ChannelID, "```cs\n"+
				"# 봇이 입장한 채널이 없습니다\n"+
				"```",
			)

			return
		}

		LeaveChannel(m.GuildID)
	case "~volume":
		if len(method) < 2 {
			s.ChannelMessageSend(m.ChannelID, "```cs\n"+
				"# 사용법: ~v 볼륨(숫자)\n"+
				"```",
			)

			return
		}

		if _, ok := voiceConnection[m.GuildID]; !ok {
			s.ChannelMessageSend(m.ChannelID, "```cs\n"+
				"# 봇이 입장한 채널이 없습니다\n"+
				"```",
			)
		}

		volume, err := strconv.Atoi(method[1])
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "```cs\n"+
				"# 숫자가 아닙니다\n"+
				"```",
			)

			return
		}

		voiceConnection[m.GuildID].VoiceOption.Volume = volume
		options.Volume = volume

		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```md\n"+
			"# 다음 곡 부터 적용됩니다 (볼륨: %d)\n"+
			"```",
			volume,
		))
	case "~speed":
		if len(method) < 2 {
			s.ChannelMessageSend(m.ChannelID, "```cs\n"+
				"# 사용법: ~s 속도(숫자)\n"+
				"```",
			)

			return
		}

		speed, err := strconv.Atoi(method[1])
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "```cs\n"+
				"# 숫자가 아닙니다\n"+
				"```",
			)

			return
		}

		options.FrameDuration = speed

		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```md\n"+
			"# 다음 곡 부터 적용됩니다 (속도: %d)\n"+
			"```",
			speed,
		))
	default:
		if codeMatch.MatchString(method[0]) {
			if _, ok := userVerificationCode[m.GuildID]; ok {
				if userVerificationCode[m.GuildID][method[0]] != nil {
					log.Printf("[%s][%s] 승인 중...", m.GuildID, userVerificationCode[m.GuildID][method[0]].IP)
					userVerification[m.GuildID][userVerificationCode[m.GuildID][method[0]].IP] <- &UserInfo{
						UserID: m.Author.ID,
					}
					log.Println("완료")
				}
			}
		}
	}
}

func OnWordChainMessage(s *discordgo.Session, m *discordgo.Message) {
	defer Recover()

	if s.State.Ready.User.Username == m.Author.Username {
		return
	}

	method := strings.Split(m.Content, " ")

	if len(method) < 1 {
		return
	}

	switch method[0] {
	case "~시작":
		log.Println("시작")

		if _, ok := users[m.Author.ID]; ok {
			delete(users, m.Author.ID)
		}

		users[m.Author.ID] = &UserInfo{
			UserID:    m.Author.ID,
			UserName:  m.Author.Username,
			StartTime: time.Now(),
			WordLog:   []string{},
			Round:     0,
			Retry:     0,
		}

		users[m.Author.ID].Status = true

		embed := &discordgo.MessageEmbed{
			Author:      &discordgo.MessageEmbedAuthor{},
			Color:       Blue,
			Title:       "끝말잇기",
			Description: "게임 시작",
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "-----",
					Value:  "유저가 먼저 시작합니다",
					Inline: true,
				},
			},
		}

		s.ChannelMessageSendEmbed(m.ChannelID, embed)
	case "~종료":
		if _, ok := users[m.Author.ID]; ok {
			delete(users, m.Author.ID)
		}

		embed := &discordgo.MessageEmbed{
			Author:      &discordgo.MessageEmbedAuthor{},
			Color:       Blue,
			Title:       "끝말잇기",
			Description: "게임 종료",
		}

		s.ChannelMessageSendEmbed(m.ChannelID, embed)
	default:
		if _, ok := users[m.Author.ID]; !ok {
			return
		}

		if users[m.Author.ID].Retry >= 5 {
			embed := &discordgo.MessageEmbed{
				Author:      &discordgo.MessageEmbedAuthor{},
				Color:       Purple,
				Title:       "끝말잇기",
				Description: "5회 이상 실패하여 봇이 승리했습니다",
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:   "-----",
						Value:  fmt.Sprintf("%d턴 진행", users[m.Author.ID].Round),
						Inline: true,
					},
				},
			}

			s.ChannelMessageSendEmbed(m.ChannelID, embed)

			delete(users, m.Author.ID)

			return
		}

		if len([]rune(m.Content)) == 1 {
			embed := &discordgo.MessageEmbed{
				Author:      &discordgo.MessageEmbedAuthor{},
				Color:       Yellow,
				Title:       "끝말잇기",
				Description: "단어는 2글자 이상이여야 합니다",
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:   "-----",
						Value:  fmt.Sprintf("%d턴 진행", users[m.Author.ID].Round),
						Inline: true,
					},
				},
			}

			s.ChannelMessageSendEmbed(m.ChannelID, embed)

			users[m.Author.ID].Retry++

			return
		}

		if len(users[m.Author.ID].PreWord) != 0 {
			firstElem := string([]rune(m.Content)[0])
			preLastElem := string([]rune(users[m.Author.ID].PreWord)[len([]rune(users[m.Author.ID].PreWord))-1:])

			if firstElem != preLastElem {
				embed := &discordgo.MessageEmbed{
					Author:      &discordgo.MessageEmbedAuthor{},
					Color:       Yellow,
					Title:       "끝말잇기",
					Description: fmt.Sprintf("'%s'로 시작해야합니다", preLastElem),
					Fields: []*discordgo.MessageEmbedField{
						{
							Name:   "-----",
							Value:  fmt.Sprintf("%d턴 진행", users[m.Author.ID].Round),
							Inline: true,
						},
					},
				}

				s.ChannelMessageSendEmbed(m.ChannelID, embed)

				users[m.Author.ID].Retry++

				return
			}
		}

		ok := CheckWord(m.Content)
		if !ok {
			embed := &discordgo.MessageEmbed{
				Author:      &discordgo.MessageEmbedAuthor{},
				Color:       Yellow,
				Title:       "끝말잇기",
				Description: fmt.Sprintf("'%s'는 사전에 존재하지 않는 단어입니다", m.Content),
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:   "-----",
						Value:  fmt.Sprintf("%d턴 진행", users[m.Author.ID].Round),
						Inline: true,
					},
				},
			}

			s.ChannelMessageSendEmbed(m.ChannelID, embed)
			users[m.Author.ID].Retry++

			return
		}

		if CheckExist(m.Content, m.Author.ID) {
			embed := &discordgo.MessageEmbed{
				Author:      &discordgo.MessageEmbedAuthor{},
				Color:       Yellow,
				Title:       "끝말잇기",
				Description: fmt.Sprintf("'%s'는 이미 사용된 단어입니다", m.Content),
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:   "-----",
						Value:  fmt.Sprintf("%d턴 진행", users[m.Author.ID].Round),
						Inline: true,
					},
				},
			}

			s.ChannelMessageSendEmbed(m.ChannelID, embed)
			users[m.Author.ID].Retry++

			return
		}

		users[m.Author.ID].Retry = 0
		lastElem := string([]rune(m.Content)[len([]rune(m.Content))-1:])

		fmt.Println(lastElem)

		word := GetWord(lastElem, m.Author.ID)

		if len(word) == 0 {
			embed := &discordgo.MessageEmbed{
				Author:      &discordgo.MessageEmbedAuthor{},
				Color:       Green,
				Title:       "끝말잇기",
				Description: "봇이 패배했습니다",
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:   "-----",
						Value:  fmt.Sprintf("%d턴 진행", users[m.Author.ID].Round),
						Inline: true,
					},
				},
			}

			s.ChannelMessageSendEmbed(m.ChannelID, embed)

			delete(users, m.Author.ID)

			return
		}

		users[m.Author.ID].PreWord = word

		embed := &discordgo.MessageEmbed{
			Author:      &discordgo.MessageEmbedAuthor{},
			Color:       Blue,
			Title:       word,
			Description: "",
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "-----",
					Value:  fmt.Sprintf("%d턴 진행", users[m.Author.ID].Round),
					Inline: true,
				},
			},
		}

		s.ChannelMessageSendEmbed(m.ChannelID, embed)

		users[m.Author.ID].WordLog = append(users[m.Author.ID].WordLog, m.Content)
		users[m.Author.ID].WordLog = append(users[m.Author.ID].WordLog, word)
		users[m.Author.ID].Round++
	}
}
