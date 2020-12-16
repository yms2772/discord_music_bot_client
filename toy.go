package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	"gopkg.in/ini.v1"

	"github.com/bwmarrin/discordgo"
	"github.com/maxence-charriere/go-app/v7/pkg/app"
)

func main() {
	var err error
	cfg, err := ini.Load("setting.ini")
	if err != nil {
		panic(err)
	}

	botToken = cfg.Section("key").Key("botToken").MustString("none")
	youtubeAPIKey = cfg.Section("key").Key("youtubeAPIKey").MustString("none")
	mysqlServer = cfg.Section("key").Key("mysqlServer").MustString("none")

	switch {
	case botToken == "none":
		panic(errors.New("'[key] -> botToken' is empty"))
	case youtubeAPIKey == "none":
		panic(errors.New("'[key] -> youtubeAPIKey' is empty"))
	case mysqlServer == "none":
		panic(errors.New("'[key] -> mysqlServer' is empty"))
	}

	codeMatch, _ = regexp.Compile(`\b\d{5}\b`)

	log.Println("디스코드 로그인 중...")
	discord, err = discordgo.New("Bot " + botToken)
	if err != nil {
		panic(err)
	}

	discord.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsAll)

	discord.AddHandler(OnUpdateStatus)
	discord.AddHandler(OnMessageUpdate)

	options.RawOutput = true
	options.Bitrate = 120
	options.Application = "lowdelay"
	options.Volume = 256

	if err := discord.Open(); err != nil {
		fmt.Println(err)
	}

	h := &app.Handler{
		Title: "Music Queue",
		Styles: []string{
			"/web/app.css",
		},
		Scripts: []string{
			"https://jsgetip.appspot.com",
		},
		Icon: app.Icon{
			Default:    "/web/favicon/192.png",
			Large:      "/web/favicon/512.png",
			AppleTouch: "/web/favicon/192.png",
		},
		ThemeColor: "#16171f",
		Version:    time.Now().Format("20060102150405"),
	}

	http.Handle("/", h)
	http.HandleFunc("/websocket/", Websocket)

	log.Println("웹 소켓 실행 중...")
	log.Fatal(http.ListenAndServeTLS(":443", "./ssl/server.cert", "./ssl/server.key", nil))
}
