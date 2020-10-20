package main

import (
	"bufio"
	"net/http"
	"time"

	"github.com/bwmarrin/discordgo"
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
}

type Channel struct {
	Done chan error
}

type VoiceConnection struct {
	VoiceOption

	GuildID          string
	VC               *discordgo.VoiceConnection
	Done             chan error
	EncodingSession  *dca.EncodeSession
	Idle             bool
	IdleTime         time.Time
	StopRelatedVideo bool
}

type VoiceOption struct {
	Volume int
}

type VideoQueue struct {
	UnixNano     int64
	ID           string
	Title        string
	Duration     int
	Thumbnail    string
	Reader       *bufio.Reader
	BufferLength int
	Response     *http.Response
}

type VideoQueueInfo struct {
	UnixNano  int64
	ID        string
	Title     string
	Duration  int
	Thumbnail string
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
