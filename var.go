package main

import (
	"net/http"
	"regexp"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"
	"github.com/jonas747/dca"
	ytClient "github.com/kkdai/youtube/v2"
)

const (
	Red    = 0xFF0000
	Blue   = 0x0000FF
	Green  = 0x00FF00
	Purple = 0x8B00FF
	Yellow = 0xFFFF00
	Brown  = 0x8B4513
	Pink   = 0xFF1493
)

var (
	botToken      string
	youtubeAPIKey string
	mysqlServer   string
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn = make(map[string]map[int64]*WebsocketConnection)
)

var (
	users                = make(map[string]*UserInfo)
	voiceConnection      = make(map[string]*VoiceConnection)
	userVerification     = make(map[string]map[string]chan *UserInfo)
	userVerificationCode = make(map[string]map[string]*UserInfo)
)

var (
	codeMatch *regexp.Regexp
	discord   *discordgo.Session
	yt        = ytClient.Client{}
	options   = dca.StdEncodeOptions
)
