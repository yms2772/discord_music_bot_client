package main

import (
	"bufio"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func OnUpdateStatus(s *discordgo.Session, _ *discordgo.Ready) {
	s.UpdateStatus(0, "일")
}

func OnMessageUpdate(s *discordgo.Session, m *discordgo.MessageCreate) {
	OnWordChainMessage(s, m.Message)
	OnMusicMessage(s, m.Message)
}

func OnMusicMessage(s *discordgo.Session, m *discordgo.Message) {
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

	if _, ok := voiceConnection[m.GuildID]; ok {
		if vcState.ChannelID != voiceConnection[m.GuildID].VC.ChannelID {
			s.ChannelMessageSend(m.ChannelID, "```cs\n"+
				"# 다른 채널에서 이미 사용중입니다\n"+
				"```",
			)

			return
		}
	}

	switch method[0] {
	case "~h", "~help":
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Author: &discordgo.MessageEmbedAuthor{
				URL:     s.State.Ready.User.AvatarURL(""),
				Name:    "사용법",
				IconURL: s.State.Ready.User.AvatarURL(""),
			},
			Color: Yellow,
			Description: fmt.Sprintf("`~p 음악 제목`: 유튜브에서 영상 재생\n\n" +
				"~pl 음악 제목`: 유튜브에서 관련 영상 이어서 재생\n\n" +
				"~c`: 유튜브 관련 영상 재생 종료\n\n" +
				"`~q`: 대기열 확인\n\n" +
				"`~fs`: 강제 건너뛰기\n\n" +
				"`~l`: 채널에서 봇 퇴장\n\n" +
				"`~v 볼륨`: 볼륨 설정\n",
			),
		})
	case "~p", "~play", "~pl", "~playlist":
		if len(method) < 2 {
			s.ChannelMessageSend(m.ChannelID, "```cs\n"+
				"# 사용법: {~p, ~pl} 제목\n"+
				"```",
			)

			return
		}

		log.Println("================================================================")
		if _, ok := voiceConnection[m.GuildID]; !ok || !voiceConnection[m.GuildID].VC.Ready {
			log.Printf("연결: %s", m.GuildID)
			channel, _ := s.Channel(vcState.ChannelID)
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🔗 `연결: %s`", channel.Name))

			done := make(chan error)

			vc, err := JoinVoiceChannel(s, vcState.ChannelID)
			if err != nil {
				fmt.Println(err)
				SendErrorMessage(s, m.ChannelID, 10000)

				return
			}

			voiceConnection[m.GuildID] = &VoiceConnection{
				VoiceOption: VoiceOption{
					Volume: 256,
				},
				GuildID: m.GuildID,
				VC:      vc,
				Done:    done,
			}

			videoQueue[m.GuildID] = make(chan *VideoQueue)

			go func() { // 재생
				log.Println("Range 시작: " + m.GuildID)
				for item := range videoQueue[m.GuildID] {
					log.Printf("Title: %s", item.Title)
					s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🎶 `재생: %s`", item.Title))

					TTSAction(voiceConnection[m.GuildID], item)

					log.Println("재생 끝")
					videoQueueInfo[m.GuildID] = RemoveQueue(m.GuildID, item.UnixNano)
					item.Response.Body.Close()
				}

				log.Println("Range 끝: " + m.GuildID)
			}()

			go func() { // IDLE 확인
				for {
					if _, ok := videoQueueInfo[m.GuildID]; ok {
						if _, ok := voiceConnection[m.GuildID]; ok {
							if voiceConnection[m.GuildID].VC.Ready {
								if len(videoQueueInfo[m.GuildID]) == 0 && !voiceConnection[m.GuildID].Idle {
									voiceConnection[m.GuildID].Idle = true
									voiceConnection[m.GuildID].IdleTime = time.Now()
								}

								if voiceConnection[m.GuildID].Idle {
									if time.Since(voiceConnection[m.GuildID].IdleTime).Minutes() > 5 {
										log.Println("대기 상태로 인해 퇴장")
										voiceConnection[m.GuildID].Idle = false
										_ = voiceConnection[m.GuildID].VC.Disconnect()
										delete(voiceConnection, m.GuildID)

										if _, ok := <-videoQueue[m.GuildID]; ok {
											close(videoQueue[m.GuildID])
										}

										s.ChannelMessageSend(m.ChannelID, "```cs\n"+
											"# 대기상태로 인해 퇴장\n"+
											"```",
										)
									}
								}
							} else {
								voiceConnection[m.GuildID].Idle = false
							}
						}
					}

					time.Sleep(time.Second)
				}
			}()
		}

		log.Println("음악 대기 중...")
		q := strings.Join(method[1:], " ")

		searching, _ := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🎵 `검색 중: %s`", q))

		list, err := GetYoutubeSearchList(q)
		if err != nil {
			_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)
			SendErrorMessage(s, m.ChannelID, 10001)

			return
		}

		if len(list.Items) < 1 {
			_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```cs\n"+
				"# 검색결과가 없습니다.\n"+
				"```",
			))

			return
		}

		errorCount := 0

		result := list.Items[errorCount]
		resp, videoDuration, err := GetYoutubeMusic(result.ID.VideoID)
		for err != nil {
			if errorCount > 5 {
				_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)
				SendErrorMessage(s, m.ChannelID, 10002)

				return
			}

			result = list.Items[errorCount]
			resp, videoDuration, err = GetYoutubeMusic(result.ID.VideoID)

			errorCount++
		}

		var totalVideo, currentVideo, cantPlay int
		var relatedList []YoutubeSearch

		switch method[0] {
		case "~pl", "~playlist":
			voiceConnection[m.GuildID].StopRelatedVideo = false

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
		}

		videoID := result.ID.VideoID
		videoTitle := html.UnescapeString(result.Snippet.Title)
		videoThumbnail := result.Snippet.Thumbnails.High.URL
		videoChannel := html.UnescapeString(result.Snippet.ChannelTitle)
		videoDurationSeconds := int(videoDuration.Seconds())

		videoDurationH := videoDurationSeconds / 3600
		videoDurationM := (videoDurationSeconds - (3600 * videoDurationH)) / 60
		videoDurationS := videoDurationSeconds - (3600 * videoDurationH) - (videoDurationM * 60)

		log.Printf("검색된 영상: %s (%s) (%d초)", videoTitle, result.ID.VideoID, videoDurationSeconds)
		log.Printf("버퍼 생성: %d bytes", resp.ContentLength)
		_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)

		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Author: &discordgo.MessageEmbedAuthor{
				URL:     "https://www.youtube.com/watch?v=" + videoID,
				Name:    "재생목록 추가",
				IconURL: m.Author.AvatarURL(""),
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text:    "Youtube",
				IconURL: "http://mokky.ipdisk.co.kr:8000/list/HDD1/icon/youtube_logo.png",
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
					Value:  fmt.Sprintf("%d", len(videoQueueInfo[m.GuildID])),
					Inline: true,
				},
			},
		})

		log.Println("대기열 전송 중...")
		unixNano := time.Now().UnixNano()

		videoQueueInfo[m.GuildID] = append(videoQueueInfo[m.GuildID], &VideoQueueInfo{
			UnixNano:  unixNano,
			ID:        videoID,
			Title:     videoTitle,
			Duration:  videoDurationSeconds,
			Thumbnail: videoThumbnail,
		})

		videoQueue[m.GuildID] <- &VideoQueue{
			UnixNano:     unixNano,
			ID:           videoID,
			Title:        videoTitle,
			Duration:     videoDurationSeconds,
			Thumbnail:    videoThumbnail,
			Reader:       bufio.NewReaderSize(resp.Body, int(resp.ContentLength)),
			BufferLength: int(resp.ContentLength),
			Response:     resp,
		}

		switch method[0] {
		case "~pl", "~playlist":
		LIST:
			for _, page := range relatedList {
			ITEM:
				for _, item := range page.Items {
					if voiceConnection[m.GuildID].StopRelatedVideo {
						s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ `종료: %s`", q))

						break LIST
					}

					errorCount = 0

					resp, videoDuration, err := GetYoutubeMusic(item.ID.VideoID)
					for err != nil {
						errorCount++

						if errorCount > 10 {
							cantPlay++
							totalVideo--

							continue ITEM
						}

						resp, videoDuration, err = GetYoutubeMusic(item.ID.VideoID)
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
					log.Printf("버퍼 생성: %d bytes", resp.ContentLength)
					_ = s.ChannelMessageDelete(m.ChannelID, searching.ID)

					s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
						Author: &discordgo.MessageEmbedAuthor{
							URL:     "https://www.youtube.com/watch?v=" + videoID,
							Name:    "재생목록 추가",
							IconURL: m.Author.AvatarURL(""),
						},
						Footer: &discordgo.MessageEmbedFooter{
							Text:    "Youtube",
							IconURL: "http://mokky.ipdisk.co.kr:8000/list/HDD1/icon/youtube_logo.png",
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
					unixNano := time.Now().UnixNano()

					videoQueueInfo[m.GuildID] = append(videoQueueInfo[m.GuildID], &VideoQueueInfo{
						UnixNano:  unixNano,
						ID:        videoID,
						Title:     videoTitle,
						Duration:  videoDurationSeconds,
						Thumbnail: videoThumbnail,
					})

					videoQueue[m.GuildID] <- &VideoQueue{
						UnixNano:     unixNano,
						ID:           videoID,
						Title:        videoTitle,
						Duration:     videoDurationSeconds,
						Thumbnail:    videoThumbnail,
						Reader:       bufio.NewReaderSize(resp.Body, int(resp.ContentLength)),
						BufferLength: int(resp.ContentLength),
						Response:     resp,
					}
				}
			}
		}

		log.Println("대기열 전송 완료")
	case "~c", "~cancel":
		voiceConnection[m.GuildID].StopRelatedVideo = true

		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```md\n"+
			"# 다음 곡 부터 적용됩니다\n"+
			"```",
		))
	case "~q", "~queue":
		var data string
		guild, _ := s.Guild(m.GuildID)

		for i, item := range videoQueueInfo[m.GuildID] {
			data += fmt.Sprintf("%d. [%s](%s)\n", i+1, item.Title, "https://www.youtube.com/watch?v="+item.ID)
		}

		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Author: &discordgo.MessageEmbedAuthor{
				URL:     guild.IconURL(),
				Name:    fmt.Sprintf("%s의 재생목록", guild.Name),
				IconURL: guild.IconURL(),
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text:    "Youtube",
				IconURL: "http://mokky.ipdisk.co.kr:8000/list/HDD1/icon/youtube_logo.png",
			},
			Color:       Pink,
			Description: data,
		})
	case "~fs", "~force_skip":
		if item, ok := videoQueueInfo[m.GuildID]; ok && len(item) != 0 {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("⏭ `건너뛰기: %s`", videoQueueInfo[m.GuildID][0].Title))

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

		_ = voiceConnection[m.GuildID].VC.Disconnect()
		delete(voiceConnection, m.GuildID)
		close(videoQueue[m.GuildID])

		s.ChannelMessageSend(m.ChannelID, "```md\n"+
			"# 퇴장\n"+
			"```",
		)
	case "~v", "~volume":
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

		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```md\n"+
			"# 다음 곡 까지만 재생됩니다 (볼륨: %d)\n"+
			"```",
			volume,
		))
	case "~ㅋ":
		vc, err := JoinVoiceChannel(s, vcState.ChannelID)
		if err != nil {
			return
		}

		TTSActionFromFile(vc, "test.mp3")

		err = vc.Disconnect()
		if err != nil {
			fmt.Println(err)
		}
		vc.Close()
	}
}

func OnWordChainMessage(s *discordgo.Session, m *discordgo.Message) {
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

		db, rows, ok := CheckWord(m.Content)
		rows.Close()
		db.Close()

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

		db, rows, ok, word := GetWord(lastElem, m.Author.ID)
		rows.Close()
		db.Close()

		if !ok {
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
