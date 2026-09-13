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

const queueSize = 16

var msgIDKey = []byte(`"message_id":`)

type TelegramService struct {
	bot       *tgbotapi.BotAPI
	channelID int64

	hc       *http.Client
	sendURL  string
	photoURL string
	meURL    string
	bodyHead string
	bodyTail string

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
		bodyHead:  fmt.Sprintf(`{"chat_id":%d,"text":`, cfg.ChannelID),
		bodyTail:  `,"parse_mode":"HTML","disable_web_page_preview":true}`,
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

// Broadcast 非阻塞入队；单 sendWorker 保证顺序与状态一致性。
func (ts *TelegramService) Broadcast(r *DrawResult) {
	select {
	case ts.queue <- r:
	default:
		log.Printf("[FAIL] %d期 发送队列已满，丢弃", r.Qihao)
	}
}

func (ts *TelegramService) sendWorker() {
	for r := range ts.queue {
		ts.sendOne(r)
	}
}

func (ts *TelegramService) sendOne(r *DrawResult) {
	message := formatMessage(r)

	start := time.Now()
	var data []byte
	var err error
	via := "图片"
	if photo, rerr := RenderDraw(r); rerr == nil {
		data, err = ts.sendPhoto(photo, message)
		if err != nil {
			log.Printf("[WARN] %d期 图片发送失败，回退纯文字: %v", r.Qihao, err)
			data, err = ts.sendText(message)
			via = "文字(回退)"
		}
	} else {
		data, err = ts.sendText(message)
		via = "文字"
	}
	if err != nil {
		log.Printf("[FAIL] %d期 TG发送失败: %v", r.Qihao, err)
		return
	}

	msgID := parseMsgID(data)
	if msgID == 0 {
		log.Printf("[FAIL] %d期 TG响应无message_id: %s", r.Qihao, data)
		return
	}

	log.Printf("[OK] %d期 TG播报完成(%s) msgID=%d [%v]", r.Qihao, via, msgID, time.Since(start))
}

func (ts *TelegramService) sendText(message string) ([]byte, error) {
	textJSON, _ := json.Marshal(message)
	body := ts.bodyHead + string(textJSON) + ts.bodyTail
	resp, err := ts.hc.Post(ts.sendURL, "application/json", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return data, nil
}

func (ts *TelegramService) sendPhoto(photo []byte, caption string) ([]byte, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("chat_id", strconv.FormatInt(ts.channelID, 10))
	_ = w.WriteField("caption", caption)
	_ = w.WriteField("parse_mode", "HTML")
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
	patternPart := ""
	if r.Pattern != "" && r.Pattern != "杂六" {
		patternPart = " " + r.Pattern
	}
	return fmt.Sprintf("🆕<b>第</b><code>%d</code><b>期</b> <code>%d+%d+%d=%02d</code> <b>%s%s%s</b>",
		r.Qihao, r.Numbers[0], r.Numbers[1], r.Numbers[2], r.Sum, r.SizeType, r.ParityType, patternPart)
}
