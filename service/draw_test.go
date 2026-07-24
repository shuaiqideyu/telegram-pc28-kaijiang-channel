package service

import (
	"encoding/json"
	"testing"
)

func TestParseKJJSON(t *testing.T) {
	raw := []byte(`{"countdown":"02:04","data":[{"nbr":"3461131","date":"2026-07-25","time":"02:12:00","number":"4+4+6","num":"14","combination":"大双"}],"message":"success"}`)
	result, err := parseKJJSON(raw)
	if err != nil {
		t.Fatalf("parseKJJSON: %v", err)
	}
	if result.Qihao != 3461131 {
		t.Fatalf("qihao=%d", result.Qihao)
	}
	if result.Numbers != [3]int{4, 4, 6} {
		t.Fatalf("numbers=%v", result.Numbers)
	}
	if result.Sum != 14 || result.SizeType != "大" || result.ParityType != "双" {
		t.Fatalf("sum/type=%d/%s/%s", result.Sum, result.SizeType, result.ParityType)
	}
	if result.Pattern != "对子" {
		t.Fatalf("pattern=%q", result.Pattern)
	}
}

func TestParseKJJSONRejectsSumMismatch(t *testing.T) {
	raw := []byte(`{"data":[{"nbr":"1","number":"1+2+3","num":"9","combination":"小双"}],"message":"success"}`)
	if _, err := parseKJJSON(raw); err == nil {
		t.Fatal("expected sum mismatch error")
	}
}

func TestBuildKeyboard(t *testing.T) {
	ts := &TelegramService{username: "fw999"}

	var keyboard struct {
		InlineKeyboard [][]keyboardButton `json:"inline_keyboard"`
	}
	if err := json.Unmarshal([]byte(ts.buildKeyboard(123)), &keyboard); err != nil {
		t.Fatalf("解析键盘 JSON 失败：%v", err)
	}
	if len(keyboard.InlineKeyboard) != 1 {
		t.Fatalf("键盘行数=%d，期望仅功能行=1", len(keyboard.InlineKeyboard))
	}

	row := keyboard.InlineKeyboard[0]
	if len(row) != 3 {
		t.Fatalf("功能按钮数=%d，期望=3", len(row))
	}
	if row[2].URL != "https://t.me/fw999/123" {
		t.Fatalf("对应按钮链接=%q", row[2].URL)
	}
}
