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
