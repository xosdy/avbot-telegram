package avbot

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"

	"golang.org/x/net/proxy"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

type AVBot struct {
	*tgbotapi.BotAPI
	hooks       []MessageHook
	client      *http.Client
	groupChatId int64
	closeCh     chan int
}

func (b *AVBot) AddMessageHook(hook MessageHook) {
	b.hooks = append(b.hooks, hook)
}

func NewBot(token string, chatId int64, socks5Addr string) *AVBot {
	dial := net.Dial
	if socks5Addr != "" {
		dialer, err := proxy.SOCKS5("tcp", socks5Addr, nil, proxy.Direct)
		if err != nil {
			panic(err)
		}
		dial = dialer.Dial
	}
	client := &http.Client{
		Transport: &http.Transport{
			Dial: dial,
		},
	}

	bot, err := tgbotapi.NewBotAPIWithClient(token, client)
	if err != nil {
		panic(err)
	}

	return &AVBot{
		BotAPI:      bot,
		hooks:       make([]MessageHook, 0, 0),
		client:      client,
		groupChatId: chatId,
		closeCh:     make(chan int),
	}
}

func (b *AVBot) Run() {

	go b.HandleSignal()

	b.Debug = true

	log.Printf("Authorized on account %s", b.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := b.GetUpdatesChan(u)
	if err != nil {
		panic(err)
	}
mainLoop:
	for {
		select {
		case update := <-updates:
			if update.Message != nil {
				b.onMessage(update.Message)
			}
		case <-b.closeCh:
			b.Stop()
			break mainLoop
		}
	}
}

func (b *AVBot) HandleSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch
	b.closeCh <- 1
	log.Println("received interrupt signal")
}

func (b *AVBot) onMessage(msg *tgbotapi.Message) {

	for _, h := range b.hooks {
		if h.Process(b, msg) {
			break
		}
	}
}

func (b *AVBot) GetBotApi() *tgbotapi.BotAPI {
	return b.BotAPI
}

func (b *AVBot) GetGroupChatId() int64 {
	return b.groupChatId
}

func (b *AVBot) Stop() {
	for _, v := range b.hooks {
		if o, ok := v.(Stoppable); ok {
			o.Stop()
		}
	}
}
