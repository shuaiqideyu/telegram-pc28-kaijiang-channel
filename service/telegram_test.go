package service

import (
	"encoding/json"
	"testing"
)

func TestBuildKeyboard(t *testing.T) {
	var kb struct {
		InlineKeyboard [][]keyboardButton `json:"inline_keyboard"`
	}
	if err := json.Unmarshal([]byte(buildKeyboard()), &kb); err != nil {
		t.Fatalf("keyboard json: %v", err)
	}
	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("rows=%d", len(kb.InlineKeyboard))
	}
	row1 := kb.InlineKeyboard[0]
	if len(row1) != 2 || row1[0].Text != "统计" || row1[0].CallbackData != statsCallback {
		t.Fatalf("row1[0]=%+v", row1[0])
	}
	if row1[1].Text != "预测开奖网" || row1[1].URL != siteURL {
		t.Fatalf("row1[1]=%+v", row1[1])
	}
	row2 := kb.InlineKeyboard[1]
	if len(row2) != 1 || row2[0].Text != "南宫集团官方频道" || row2[0].URL != nangongURL {
		t.Fatalf("row2[0]=%+v", row2[0])
	}
}

func TestFormatMessageOmitsZaliu(t *testing.T) {
	msg := formatMessage(&DrawResult{
		Qihao: 3461131, Numbers: [3]int{4, 8, 2}, Sum: 14,
		SizeType: "小", ParityType: "双", Pattern: "杂六",
	})
	want := "🆕<b>第</b><code>3461131</code><b>期</b> <code>4+8+2=14</code> <b>小双</b>"
	if msg != want {
		t.Fatalf("got=%q want=%q", msg, want)
	}
}
