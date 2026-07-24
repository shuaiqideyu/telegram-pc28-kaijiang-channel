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
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	stateFile             = "last_msg.json"
	emptyKBJSON           = `{"inline_keyboard":[]}`
	queueSize             = 16
	fallbackCorrespondURL = "https://t.me/fw999"
)

var msgIDKey = []byte(`"message_id":`)

type keyboardButton struct {
	Text         string `json:"text"`
	Style        string `json:"style,omitempty"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

type sumState struct {
	MsgID int `json:"msg_id"`
}

type persistedState struct {
	MessageID int                 `json:"message_id"`
	Sums      map[string]sumState `json:"sums,omitempty"`
}

type TelegramService struct {
	bot       *tgbotapi.BotAPI
	channelID int64
	username  string

	hc       *http.Client
	sendURL  string
	photoURL string
	rmkbURL  string
	meURL    string
	bodyHead string
	bodyMid  string
	bodyTail string

	queue chan *DrawResult

	stateMu   sync.Mutex
	lastMsgID int
	sums      map[int]sumState
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
		username:  cfg.ChannelUsername,
		hc:        &http.Client{Transport: tp},
		sendURL:   base + "/sendMessage",
		photoURL:  base + "/sendPhoto",
		rmkbURL:   base + "/editMessageReplyMarkup",
		meURL:     base + "/getMe",
		bodyHead:  fmt.Sprintf(`{"chat_id":%d,"text":`, cfg.ChannelID),
		bodyMid:   `,"parse_mode":"HTML","disable_web_page_preview":true,"reply_markup":`,
		bodyTail:  `}`,
		queue:     make(chan *DrawResult, queueSize),
		sums:      make(map[int]sumState),
	}
	ts.loadState()
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
	ts.stateMu.Lock()
	prevMsgID := ts.lastMsgID
	correspondMsgID := 0
	if s, ok := ts.sums[r.Sum]; ok {
		correspondMsgID = s.MsgID
	}
	ts.stateMu.Unlock()

	message := formatMessage(r)
	keyboardJSON := ts.buildKeyboard(correspondMsgID)

	start := time.Now()
	var data []byte
	var err error
	via := "图片"
	if photo, rerr := RenderDraw(r); rerr == nil {
		data, err = ts.sendPhoto(photo, message, keyboardJSON)
		if err != nil {
			log.Printf("[WARN] %d期 图片发送失败，回退纯文字: %v", r.Qihao, err)
			data, err = ts.sendText(message, keyboardJSON)
			via = "文字(回退)"
		}
	} else {
		data, err = ts.sendText(message, keyboardJSON)
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

	ts.stateMu.Lock()
	ts.lastMsgID = msgID
	ts.sums[r.Sum] = sumState{MsgID: msgID}
	snapshot := ts.snapshotStateLocked()
	ts.stateMu.Unlock()
	ts.persistState(snapshot)

	log.Printf("[OK] %d期 TG播报完成(%s) msgID=%d [%v]", r.Qihao, via, msgID, time.Since(start))
	if prevMsgID != 0 {
		go ts.clearOldKeyboard(prevMsgID)
	}
}

func (ts *TelegramService) sendText(message, keyboardJSON string) ([]byte, error) {
	textJSON, _ := json.Marshal(message)
	body := ts.bodyHead + string(textJSON) + ts.bodyMid + keyboardJSON + ts.bodyTail
	resp, err := ts.hc.Post(ts.sendURL, "application/json", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return data, nil
}

func (ts *TelegramService) sendPhoto(photo []byte, caption, keyboardJSON string) ([]byte, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("chat_id", strconv.FormatInt(ts.channelID, 10))
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
	patternPart := ""
	if r.Pattern != "" && r.Pattern != "杂六" {
		patternPart = " " + r.Pattern
	}
	return fmt.Sprintf("🆕<b>第</b><code>%d</code><b>期</b> <code>%d+%d+%d=%02d</code> <b>%s%s%s</b>",
		r.Qihao, r.Numbers[0], r.Numbers[1], r.Numbers[2], r.Sum, r.SizeType, r.ParityType, patternPart)
}

func (ts *TelegramService) buildKeyboard(correspondMsgID int) string {
	correspondURL := fallbackCorrespondURL
	if ts.username != "" {
		if correspondMsgID > 0 {
			correspondURL = fmt.Sprintf("https://t.me/%s/%d", ts.username, correspondMsgID)
		} else {
			correspondURL = "https://t.me/" + ts.username
		}
	}
	rows := [][]keyboardButton{{
		{Text: "📊 遗漏", Style: "danger", CallbackData: "yilou_query"},
		{Text: "🔥 统计", Style: "success", CallbackData: "tongji_query"},
		{Text: "🫆对应", Style: "primary", URL: correspondURL},
	}}
	keyboardJSON, _ := json.Marshal(struct {
		InlineKeyboard [][]keyboardButton `json:"inline_keyboard"`
	}{InlineKeyboard: rows})
	return string(keyboardJSON)
}

func (ts *TelegramService) clearOldKeyboard(prevMsgID int) {
	body := fmt.Sprintf(`{"chat_id":%d,"message_id":%d,"reply_markup":%s}`,
		ts.channelID, prevMsgID, emptyKBJSON)
	if resp, err := ts.hc.Post(ts.rmkbURL, "application/json", strings.NewReader(body)); err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func (ts *TelegramService) snapshotStateLocked() persistedState {
	s := persistedState{
		MessageID: ts.lastMsgID,
		Sums:      make(map[string]sumState, len(ts.sums)),
	}
	for k, v := range ts.sums {
		s.Sums[strconv.Itoa(k)] = v
	}
	return s
}

// persistState 临时文件 + rename 原子覆盖。
func (ts *TelegramService) persistState(s persistedState) {
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	tmp := stateFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return
	}
	_ = os.Rename(tmp, stateFile)
}

// loadState 兼容仅含 message_id 的旧格式。
func (ts *TelegramService) loadState() {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return
	}
	var s persistedState
	if err := json.Unmarshal(data, &s); err == nil && (s.MessageID != 0 || len(s.Sums) > 0) {
		ts.lastMsgID = s.MessageID
		for k, v := range s.Sums {
			if sum, err := strconv.Atoi(k); err == nil {
				ts.sums[sum] = v
			}
		}
		log.Printf("恢复状态: msgID=%d, sum 映射 %d 条", s.MessageID, len(ts.sums))
		return
	}
	if id := parseMsgID(data); id > 0 {
		ts.lastMsgID = id
		log.Printf("恢复上次消息状态 (msgID=%d, fallback)", id)
	}
}

func (ts *TelegramService) StartUpdateHandler() {
	ts.bot.MakeRequest("deleteWebhook", tgbotapi.Params{"drop_pending_updates": "true"})
	clearReq := tgbotapi.NewUpdate(-1)
	clearReq.Timeout = 0
	if updates, err := ts.bot.GetUpdates(clearReq); err == nil && len(updates) > 0 {
		log.Printf("清除 %d 条积压更新", len(updates))
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := ts.bot.GetUpdatesChan(u)
	log.Println("开始监听Telegram更新...")

	for update := range updates {
		if update.CallbackQuery == nil {
			continue
		}
		switch update.CallbackQuery.Data {
		case "yilou_query":
			ts.handleYilouQuery(update.CallbackQuery)
		case "tongji_query":
			ts.handleTongjiQuery(update.CallbackQuery)
		}
	}
}

func (ts *TelegramService) handleYilouQuery(callback *tgbotapi.CallbackQuery) {
	cache := GetYilouCache()
	if cache == nil {
		ts.answerAlert(callback.ID, "遗漏数据服务未就绪")
		return
	}
	m, err := cache.GetData()
	if err != nil {
		ts.answerAlert(callback.ID, fmt.Sprintf("获取遗漏数据失败: %v", err))
		return
	}
	ts.answerAlert(callback.ID, FormatYilouMessage(m))
}

func (ts *TelegramService) handleTongjiQuery(callback *tgbotapi.CallbackQuery) {
	msg, err := FormatTongjiMessage()
	if err != nil {
		ts.answerAlert(callback.ID, fmt.Sprintf("获取统计失败: %v", err))
		return
	}
	ts.answerAlert(callback.ID, msg)
}

func (ts *TelegramService) answerAlert(callbackID, text string) {
	alert := tgbotapi.NewCallback(callbackID, text)
	alert.ShowAlert = true
	ts.bot.Request(alert)
}
