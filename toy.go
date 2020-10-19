package main

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

func main() {
	log.Println("Logging in...")
	discord, err := discordgo.New("Bot " + BotToken)
	if err != nil {
		panic(err)
	}

	discord.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsAll)

	discord.AddHandler(OnUpdateStatus)
	discord.AddHandler(OnMessageUpdate)

	err = discord.Open()
	if err != nil {
		panic(err)
	}

	log.Println("Listening...")
	lock := make(chan bool)
	<-lock
}
