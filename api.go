package main

import (
	"encoding/json"
	"html"
	"log"
	"net/http"
	url2 "net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func Websocket(w http.ResponseWriter, r *http.Request) {
	defer Recover()

	var err error

	guildID := strings.TrimPrefix(r.URL.Path, "/websocket/")

	if len(guildID) == 0 {
		ReturnError(w, http.StatusBadRequest, "bad request")

		return
	}

	keepAliveTicker := time.NewTicker(10 * time.Second)
	clientID := time.Now().UnixNano()

	if _, ok := conn[guildID]; !ok {
		conn[guildID] = make(map[int64]*WebsocketConnection)
	}

	if _, ok := conn[guildID][clientID]; !ok {
		conn[guildID][clientID] = &WebsocketConnection{
			Conn:     nil,
			Verified: false,
		}
	}

	conn[guildID][clientID].Conn, err = upgrader.Upgrade(w, r, nil)
	if err != nil {
		ReturnError(w, http.StatusBadRequest, "websocket connection not established")

		return
	}

	defer func() {
		log.Printf("[%s][%d] websocket 종료", guildID, clientID)
		conn[guildID][clientID].Conn.Close()
		delete(conn[guildID], clientID)
		keepAliveTicker.Stop()
	}()

	go func() {
		for range keepAliveTicker.C {
			if !conn[guildID][clientID].Verified {
				continue
			}

			if err := SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
				"type": "keep_alive",
			}); err != nil {
				continue
			}
		}
	}()

EXIT:
	for {
		_, message, err := conn[guildID][clientID].Conn.ReadMessage()
		if err != nil {
			log.Println(err)
			break EXIT
		}

		var receive Receive
		json.Unmarshal(message, &receive)

		if receive.AccessToken != GetAccessToken(receive.RefreshToken) {
			if receive.AccessToken == GetBeforeAccessToken(receive.RefreshToken) {
				_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
					"type": "token_refresh_required",
				})

				goto API
			}

			_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
				"type": "token_expired",
			})

			continue
		}

	API:
		switch receive.Type {
		case "verify":
			if ok, _ := VerifyUser(receive.RefreshToken); !ok {
				_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
					"type": "verify_not_done",
				})

				continue
			}

			conn[guildID][clientID].Verified = true

			_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
				"type": "verify_done",
			})
		case "verify_user":
			ok, userID := VerifyUser(receive.RefreshToken)

			_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
				"type":    "verify",
				"verify":  ok,
				"user_id": userID,
			})
		case "add_verify_user":
			log.Printf("디스코드 인증: %s (%s)", receive.IP, receive.GuildID)
			_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
				"type":   "add_verify",
				"verify": AddVerifyUser(receive.GuildID, receive.IP, receive.Code),
			})
		case "channel_join_status":
			status := true

			if voiceConnection[guildID] == nil || len(voiceConnection[guildID].ChannelID) == 0 {
				status = false
			}

			_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
				"type":                "channel_join_status",
				"channel_join_status": status,
			})
		case "play_jump":
			if ok, _ := VerifyUser(receive.RefreshToken); !ok {
				_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
					"type": "require_verify",
				})

				continue
			}

			voiceConnection[guildID].StartTime <- receive.StartTime
		case "play_pause":
			if ok, _ := VerifyUser(receive.RefreshToken); !ok {
				_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
					"type": "require_verify",
				})

				continue
			}

			if voiceConnection[guildID].StreamSession == nil {
				continue EXIT
			}

			voiceConnection[guildID].StreamSession.SetPaused(!voiceConnection[guildID].StreamSession.Paused())
		case "add_queue":
			if ok, _ := VerifyUser(receive.RefreshToken); !ok {
				_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
					"type": "require_verify",
				})

				continue
			}

			var videoDuration time.Duration
			var list YoutubeSearch
			var videoID, videoTitle, videoThumbnail, videoChannel string
			var videoDurationSeconds int

			urlParse, err := url2.Parse(receive.Search)
			if err != nil {
				_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
					"type": "alert",
					"msg":  "검색 실패\n잠시 후 다시 시도해주세요",
				})

				continue EXIT
			}

			switch urlParse.Host {
			case "youtube.com", "www.youtube.com":
				log.Println("Full URL 검색")
				urlQuery, _ := url2.ParseQuery(urlParse.RawQuery)
				id := urlQuery["v"]

				if len(id) == 0 {
					_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
						"type": "alert",
						"msg":  "검색 실패\n잠시 후 다시 시도해주세요",
					})

					continue EXIT
				}

				list, err = GetYoutubeVideoInfo(id[0])
				if err != nil {
					_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
						"type": "alert",
						"msg":  "검색 실패\n잠시 후 다시 시도해주세요",
					})

					continue EXIT
				}

				videoID = id[0]
				log.Printf("Video ID: %s", videoID)
			case "youtu.be", "www.youtu.be":
				log.Println("단축 URL 검색")
				list, err = GetYoutubeVideoInfo(urlParse.Path[1:])
				if err != nil {
					_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
						"type": "alert",
						"msg":  "검색 실패\n잠시 후 다시 시도해주세요",
					})

					continue EXIT
				}

				videoID = urlParse.Path[1:]
				log.Printf("Video ID: %s", videoID)
			default:
				list, err = GetYoutubeSearchList(receive.Search)
				if err != nil {
					_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
						"type": "alert",
						"msg":  "검색 실패\n잠시 후 다시 시도해주세요",
					})

					continue EXIT
				}
			}

			if len(list.Items) < 1 {
				log.Println("Item Size: 0")
				_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
					"type": "alert",
					"msg":  "검색 실패\n잠시 후 다시 시도해주세요",
				})

				continue EXIT
			}

			errorCount := 0
			result := list.Items[errorCount]

			if len(videoID) == 0 {
				videoDuration, err = GetYoutubeMusicDuration(result.ID.VideoID)
				for err != nil {
					errorCount++

					if errorCount > 5 {
						_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
							"type": "alert",
							"msg":  "검색 실패\n잠시 후 다시 시도해주세요",
						})

						continue EXIT
					}

					if len(list.Items) == errorCount {
						_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
							"type": "alert",
							"msg":  "검색 실패\n잠시 후 다시 시도해주세요",
						})

						continue EXIT
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
					_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
						"type": "alert",
						"msg":  "검색 실패\n잠시 후 다시 시도해주세요",
					})

					continue EXIT
				}

				videoTitle = html.UnescapeString(result.Snippet.Title)
				videoThumbnail = result.Snippet.Thumbnails.High.URL
				videoChannel = html.UnescapeString(result.Snippet.ChannelTitle)
				videoDurationSeconds = int(videoDuration.Seconds())
			}

			_ = AddQueue(&VideoQueue{
				GuildID:   guildID,
				ID:        videoID,
				Channel:   videoChannel,
				Title:     videoTitle,
				Duration:  videoDurationSeconds,
				Thumbnail: videoThumbnail,
			})

			_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
				"type": "loading_done",
				"msg":  "대기열에 추가되었습니다",
			})
			_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
				"type": "add_queue_done",
			})
		case "queue_list":
			if ok, _ := VerifyUser(receive.RefreshToken); !ok {
				_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
					"type": "require_verify",
				})

				continue
			}

			var videoQueue []*VideoQueue

			for _, item := range GetVideoQueue(guildID) {
				videoQueue = append(videoQueue, &VideoQueue{
					QueueID:   item.QueueID,
					GuildID:   item.GuildID,
					ID:        item.ID,
					Title:     item.Title,
					Channel:   item.Channel,
					Duration:  item.Duration,
					Thumbnail: item.Thumbnail,
				})
			}

			_ = SendWebsocket(websocket.TextMessage, guildID, 0, map[string]interface{}{
				"type":             "update_queue",
				"video_queue_info": videoQueue,
			})
		case "queue_delete":
			if ok, _ := VerifyUser(receive.RefreshToken); !ok {
				_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
					"type": "require_verify",
				})

				continue
			}

			_ = RemoveQueue(receive.GuildID, receive.QueueID)
			_ = SendWebsocket(websocket.TextMessage, guildID, 0, map[string]interface{}{
				"type": "loading_done",
			})
		case "queue_skip":
			if ok, _ := VerifyUser(receive.RefreshToken); !ok {
				_ = SendWebsocket(websocket.TextMessage, guildID, clientID, map[string]interface{}{
					"type": "require_verify",
				})

				continue
			}

			TTSSkip(receive.GuildID)
			_ = SendWebsocket(websocket.TextMessage, guildID, 0, map[string]interface{}{
				"type": "loading_done",
			})
		}
	}
}
