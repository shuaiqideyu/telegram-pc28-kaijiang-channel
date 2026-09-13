package service

import (
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

func TestParseKJJSONYu28SumForm(t *testing.T) {
	raw := []byte(`{"countdown":"02:40","data":[{"nbr":"3463701","time":"2026-07-31 11:25:00","number":"4+8+2=14","combination":"小双"}],"message":"success"}`)
	result, err := parseKJJSON(raw)
	if err != nil {
		t.Fatalf("parseKJJSON yu28: %v", err)
	}
	if result.Qihao != 3463701 {
		t.Fatalf("qihao=%d", result.Qihao)
	}
	if result.Numbers != [3]int{4, 8, 2} || result.Sum != 14 {
		t.Fatalf("numbers/sum=%v/%d", result.Numbers, result.Sum)
	}
	if result.SizeType != "小" || result.ParityType != "双" {
		t.Fatalf("combination=%s%s", result.SizeType, result.ParityType)
	}
}

func TestParseKJJSONRejectsSumMismatch(t *testing.T) {
	raw := []byte(`{"data":[{"nbr":"1","number":"1+2+3","num":"9","combination":"小双"}],"message":"success"}`)
	if _, err := parseKJJSON(raw); err == nil {
		t.Fatal("expected sum mismatch error")
	}
}

func TestParseKJJSONRejectsYu28SumMismatch(t *testing.T) {
	raw := []byte(`{"data":[{"nbr":"1","number":"1+2+3=9","combination":"小双"}],"message":"success"}`)
	if _, err := parseKJJSON(raw); err == nil {
		t.Fatal("expected yu28 sum mismatch error")
	}
}

func TestFormatMissStatsPrefersPaddedKeys(t *testing.T) {
	got := formatMissStats(map[string]int{
		"00": 6655, "0": 1,
		"27": 2558,
		"01": 2411, "1": 2,
		"26": 1999,
		"13": 2,
		"14": 29,
		"极大": 17,
		"极小": 10,
		"豹子": 32,
	})
	want := "PC28未开统计\n\n0:6655\n27:2558\n\n1:2411\n26:1999\n\n13:2\n14:29\n\n极大:17\n极小:10\n豹子:32"
	if got != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
}

func TestFormatMissStatsFallsBackToBareKeys(t *testing.T) {
	got := formatMissStats(map[string]int{
		"0": 3, "27": 4, "1": 5, "26": 6, "13": 7, "14": 8,
		"极大": 9, "极小": 10, "豹子": 11,
	})
	want := "PC28未开统计\n\n0:3\n27:4\n\n1:5\n26:6\n\n13:7\n14:8\n\n极大:9\n极小:10\n豹子:11"
	if got != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
}
