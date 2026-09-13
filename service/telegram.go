package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"kan28/config"
	"log"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	queueSize     = 16
	siteURL       = "https://pcddkj.com/"
	nangongURL    = "https://t.me/ng99"
	statsCallback = "stats_query"
)

var msgIDKey = []byte(`"message_id":`)

type keyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

type TelegramService struct {
	bot       *tgbotapi.BotAPI
	channelID int64

	hc       *http.Client
	sendURL  string
	photoURL string
	meURL    string

	queue chan *DrawResult
}

func NewTelegramService(cfg *config.Config) (*TelegramService, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		return nil, fmt.Errorf("初始化Telegram Bot失败: %v", err)
	}
	log.Printf("Telegram Bot授权成功: @%s", bot.Self.UserName)

	tgIP := resolveDNS("api.telegram.org")
	tp := pinnedTransport(tgIP)
	tp.MaxIdleConnsPerHost = 3
	tp.IdleConnTimeout = 180 * time.Second
	log.Printf("[TG] HTTP客户端就绪 (IP: %s)", tgIP)

	base := "https://api.telegram.org/bot" + cfg.BotToken
	ts := &TelegramService{
		bot:       bot,
		channelID: cfg.ChannelID,
		hc:        &http.Client{Transport: tp},
		sendURL:   base + "/sendMessage",
		photoURL:  base + "/sendPhoto",
		meURL:     base + "/getMe",
		queue:     make(chan *DrawResult, queueSize),
	}
	ts.ping()
	go ts.keepAlive()
	go ts.sendWorker()
	return ts, nil
}

func (ts *TelegramService) ping() {
	if resp, err := ts.hc.Get(ts.meURL); err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func (ts *TelegramService) keepAlive() {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for range t.C {
		ts.ping()
	}
}

func parseMsgID(data []byte) int {
	idx := bytes.Index(data, msgIDKey)
	if idx < 0 {
		return 0
	}
	i := idx + len(msgIDKey)
	id := 0
	for i < len(data) && data[i] >= '0' && data[i] <= '9' {
		id = id*10 + int(data[i]-'0')
		i++
	}
	return id
}

// Broadcast 非阻塞入队频道播报；单 sendWorker 保证顺序。
func (ts *TelegramService) Broadcast(r *DrawResult) {
	select {
	case ts.queue <- r:
	default:
		log.Printf("[FAIL] %d期 发送队列已满，丢弃", r.Qihao)
	}
}

func (ts *TelegramService) sendWorker() {
	for r := range ts.queue {
		ts.sendOneTo(ts.channelID, r)
	}
}

func (ts *TelegramService) sendOneTo(chatID int64, r *DrawResult) {
	message := formatMessage(r)
	keyboardJSON := buildKeyboard()

	start := time.Now()
	var data []byte
	var err error
	via := "图片"
	if photo, rerr := RenderDraw(r); rerr == nil {
		data, err = ts.sendPhoto(chatID, photo, message, keyboardJSON)
		if err != nil {
			log.Printf("[WARN] %d期 图片发送失败，回退纯文字: %v", r.Qihao, err)
			data, err = ts.sendText(chatID, message, keyboardJSON)
			via = "文字(回退)"
		}
	} else {
		data, err = ts.sendText(chatID, message, keyboardJSON)
		via = "文字"
	}
	if err != nil {
		log.Printf("[FAIL] %d期 TG发送失败 chat=%d: %v", r.Qihao, chatID, err)
		return
	}

	msgID := parseMsgID(data)
	if msgID == 0 {
		log.Printf("[FAIL] %d期 TG响应无message_id chat=%d: %s", r.Qihao, chatID, data)
		return
	}

	log.Printf("[OK] %d期 TG播报完成(%s) chat=%d msgID=%d [%v]", r.Qihao, via, chatID, msgID, time.Since(start))
}

func (ts *TelegramService) sendText(chatID int64, message, keyboardJSON string) ([]byte, error) {
	textJSON, _ := json.Marshal(message)
	body := fmt.Sprintf(`{"chat_id":%d,"text":%s,"parse_mode":"HTML","disable_web_page_preview":true,"reply_markup":%s}`,
		chatID, textJSON, keyboardJSON)
	resp, err := ts.hc.Post(ts.sendURL, "application/json", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return data, nil
}

func (ts *TelegramService) sendPhoto(chatID int64, photo []byte, caption, keyboardJSON string) ([]byte, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("chat_id", strconv.FormatInt(chatID, 10))
	_ = w.WriteField("caption", caption)
	_ = w.WriteField("parse_mode", "HTML")
	_ = w.WriteField("reply_markup", keyboardJSON)
	fw, err := w.CreateFormFile("photo", "draw.jpg")
	if err != nil {
		return nil, err
	}
	if _, err := fw.Write(photo); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	resp, err := ts.hc.Post(ts.photoURL, w.FormDataContentType(), &buf)
	if err != nil {
		return nil, err
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !bytes.Contains(data, []byte(`"ok":true`)) {
		return data, fmt.Errorf("sendPhoto 响应异常: %s", data)
	}
	return data, nil
}

func formatMessage(r *DrawResult) string {
	pattern := strings.TrimSpace(r.Pattern)
	if pattern != "" {
		pattern = " " + pattern
	}
	return fmt.Sprintf("<b>📢%d期 %d+%d+%d=%d %s%s%s</b>",
		r.Qihao, r.Numbers[0], r.Numbers[1], r.Numbers[2], r.Sum, r.SizeType, r.ParityType, pattern)
}

func buildKeyboard() string {
	rows := [][]keyboardButton{
		{
			{Text: "统计", CallbackData: statsCallback},
			{Text: "预测开奖网", URL: siteURL},
		},
		{
			{Text: "南宫集团官方频道", URL: nangongURL},
		},
	}
	keyboardJSON, _ := json.Marshal(struct {
		InlineKeyboard [][]keyboardButton `json:"inline_keyboard"`
	}{InlineKeyboard: rows})
	return string(keyboardJSON)
}

func (ts *TelegramService) StartUpdateHandler() {
	ts.bot.MakeRequest("deleteWebhook", tgbotapi.Params{"drop_pending_updates": "true"})
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := ts.bot.GetUpdatesChan(u)
	log.Println("开始监听Telegram更新...")

	for update := range updates {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("[TG] update panic: %v", rec)
				}
			}()
			ts.handleUpdate(update)
		}()
	}
}

func (ts *TelegramService) handleUpdate(update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		if update.CallbackQuery.Data == statsCallback {
			ts.handleStatsQuery(update.CallbackQuery)
		}
		return
	}
	msg := update.Message
	if msg == nil || msg.Chat == nil || !msg.Chat.IsPrivate() {
		return
	}
	if strings.TrimSpace(msg.Text) != "1" {
		return
	}
	r, err := FetchLatestDraw()
	if err != nil {
		log.Printf("[调试] 拉最新开奖失败: %v", err)
		ts.replyPlain(msg.Chat.ID, "暂时拉不到开奖，请稍后再试")
		return
	}
	ts.sendOneTo(msg.Chat.ID, r)
}

func (ts *TelegramService) handleStatsQuery(callback *tgbotapi.CallbackQuery) {
	m, err := fetchYLMap()
	if err != nil {
		ts.answerAlert(callback.ID, "统计暂不可用，请稍后再试")
		return
	}
	ts.answerAlert(callback.ID, formatMissStats(m))
}

func (ts *TelegramService) answerAlert(callbackID, text string) {
	alert := tgbotapi.NewCallback(callbackID, text)
	alert.ShowAlert = true
	if _, err := ts.bot.Request(alert); err != nil {
		log.Printf("[TG] 统计弹窗失败: %v", err)
	}
}

func (ts *TelegramService) replyPlain(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := ts.bot.Send(msg); err != nil {
		log.Printf("[TG] 纯文字回执失败: %v", err)
	}
}
