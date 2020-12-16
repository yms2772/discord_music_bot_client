package main

import (
	"bufio"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"
	"github.com/jonas747/dca"
)

type UserInfo struct {
	UserID    string
	UserName  string
	StartTime time.Time
	LastTime  time.Time
	PreWord   string
	WordLog   []string
	Round     int
	Retry     int
	Status    bool
	IP        string
}

type VoiceConnection struct {
	VoiceOption

	GuildID          string
	ChannelID        string
	VC               *discordgo.VoiceConnection
	StreamSession    *dca.StreamingSession
	Idle             bool
	IdleTime         time.Time
	StopRelatedVideo bool
	IdleCheck        bool
	QueueStatus      bool
	Reader           *bufio.Reader
	Done             chan error
	StartTime        chan int
}

type WebsocketConnection struct {
	Conn     *websocket.Conn
	Verified bool
}

type VoiceOption struct {
	Volume int
}

type VideoQueue struct {
	QueueID   int
	GuildID   string
	ID        string
	Title     string
	Channel   string
	Duration  int
	Thumbnail string
}

type GetVideoQueueAPI struct {
	QueueID   int    `json:"queue_id"`
	GuildID   string `json:"guild_id"`
	ID        string `json:"id"`
	Title     string `json:"title"`
	Channel   string `json:"channel"`
	Duration  int    `json:"duration"`
	Thumbnail string `json:"thumbnail"`
	StartTime int    `json:"start_time"`
}

type Receive struct {
	GetVideoQueueAPI

	Type         string `json:"type"`
	Search       string `json:"search"`
	IP           string `json:"ip"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Code         string `json:"code"`
}

type YoutubeSearch struct {
	NextPageToken string `json:"nextPageToken"`
	Items         []struct {
		ID struct {
			Kind    string `json:"kind"`
			VideoID string `json:"videoId"`
		} `json:"id"`
		Snippet struct {
			ChannelID            string `json:"channelId"`
			ChannelTitle         string `json:"channelTitle"`
			Description          string `json:"description"`
			LiveBroadcastContent string `json:"liveBroadcastContent"`
			PublishTime          string `json:"publishTime"`
			PublishedAt          string `json:"publishedAt"`
			Thumbnails           struct {
				High struct {
					Height int64  `json:"height"`
					URL    string `json:"url"`
					Width  int64  `json:"width"`
				} `json:"high"`
			} `json:"thumbnails"`
			Title string `json:"title"`
		} `json:"snippet"`
	} `json:"items"`
}
