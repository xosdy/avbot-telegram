package stat

//go:generate protoc data.proto --go_out=.

import (
	"fmt"
	"io/ioutil"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	avbot "github.com/hyqhyq3/avbot-telegram"

	"gopkg.in/telegram-bot-api.v4"
)

type StatHook struct {
	filename string
	Groups   map[int64]*Group
	Changed  bool
	closeCh  chan int
}

func New(filename string) (h *StatHook) {

	h = &StatHook{}
	h.Groups = make(map[int64]*Group)
	h.filename = filename

	b, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Println(err)
		return
	}

	store := &Store{}
	err = proto.Unmarshal(b, store)
	if err != nil {
		log.Println(err)
		return
	}

	h.Groups = store.Groups
	h.closeCh = make(chan int)

	go h.StartLoop()

	return
}

func (h *StatHook) StartLoop() {
mainLoop:
	for {
		select {
		case <-time.After(time.Second * 60):
			h.Save(false)
		case <-h.closeCh:
			break mainLoop
		}
	}
}

func (h *StatHook) Process(bot *avbot.AVBot, msg *tgbotapi.Message) (processed bool) {
	if msg != nil {
		h.Inc(msg.Chat, msg.From)
		h.Changed = true
	}
	cmd := strings.Split(msg.Text, " ")
	if cmd[0] == "/stat" || cmd[0] == "/stat@"+bot.Self.UserName {
		mymsg := tgbotapi.NewMessage(msg.Chat.ID, h.GetStat(msg.Chat.ID))
		bot.Send(mymsg)
	}
	return false
}

type Users []*User

func (u Users) Swap(i, j int) {
	t := u[i]
	u[i] = u[j]
	u[j] = t
}

func (u Users) Less(i, j int) bool {
	return u[i].Count > u[j].Count
}

func (u Users) Len() int {
	return len(u)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (h *StatHook) GetStat(id int64) string {
	data := make([]*User, 0)
	for _, v := range h.Groups[id].Users {
		data = append(data, v)
	}
	sort.Sort(Users(data))

	var str = ""
	for i := 0; i < min(10, len(data)); i++ {
		str = str + fmt.Sprintf("%s %s: %d\n", data[i].FirstName, data[i].LastName, data[i].Count)
	}
	return str
}

func (h *StatHook) Inc(chat *tgbotapi.Chat, user *tgbotapi.User) {
	var chatid = chat.ID
	var uid = int64(user.ID)
	if _, ok := h.Groups[chatid]; !ok {
		h.Groups[chatid] = &Group{Users: make(map[int64]*User)}
	}
	if _, ok := h.Groups[chatid].Users[uid]; !ok {
		h.Groups[chatid].Users[uid] = &User{}
	}
	h.Groups[chatid].Users[uid].FirstName = user.FirstName
	h.Groups[chatid].Users[uid].LastName = user.LastName
	h.Groups[chatid].Users[uid].UserName = user.UserName
	h.Groups[chatid].Users[uid].Count++
}

func (h *StatHook) Save(force bool) {
	if !h.Changed && !force {
		return
	}
	store := &Store{}
	store.Groups = h.Groups

	b, err := proto.Marshal(store)
	if err != nil {
		log.Println(err)
		return
	}

	err = ioutil.WriteFile(h.filename, b, 0755)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("save stat data")
}

func (h *StatHook) Stop() {
	h.closeCh <- 1
	h.Save(true)
}
