package main

const (
	Red    = 0xFF0000
	Blue   = 0x0000FF
	Green  = 0x00FF00
	Purple = 0x8B00FF
	Yellow = 0xFFFF00
	Brown  = 0x8B4513
	Pink   = 0xFF1493
)

const (
	BotToken      = "NzQxOTgyODA2Nzc5MTAxMjA1.Xy_fVg.6eUkayGy9FmjlNSV57arn9hO1JE"
	YoutubeAPIKey = "AIzaSyCqa-Nek41ClErrgHH5cWSmACZNfE3RqEA"
)

var (
	users           = make(map[string]*UserInfo)
	voiceConnection = make(map[string]*VoiceConnection)
	videoQueue      = make(map[string]chan *VideoQueue)
	videoQueueInfo  = make(map[string][]*VideoQueueInfo)
)
