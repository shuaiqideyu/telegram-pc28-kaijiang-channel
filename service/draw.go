package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	kjURL        = "https://pc28.help/api/kj.json"
	ylURL        = "https://pc28.help/api/yl.json"
	ykURL        = "https://pc28.help/api/yk.json"
	pollInterval = 500 * time.Millisecond
)

// DrawResult 单期开奖结果。
type DrawResult struct {
	Qihao      int
	Numbers    [3]int
	Sum        int
	SizeType   string // 大/小
	ParityType string // 单/双
	Pattern    string // 豹子/对子/顺子/杂六
}

var pc28Client *http.Client

// InitPC28 初始化 pc28.help 共享 HTTP 客户端（开奖/遗漏/统计共用）。
func InitPC28() {
	ip := resolveDNS("pc28.help")
	pc28Client = &http.Client{Timeout: 5 * time.Second, Transport: pinnedTransport(ip)}
	if _, err := httpGet(pc28Client, kjURL); err != nil {
		log.Printf("[开奖] 预热失败: %v", err)
	}
	log.Printf("[开奖] pc28.help 客户端就绪 (IP: %s)", ip)
}

// FetchLatestDraw 启动时获取当前最新期号。
func FetchLatestDraw() (*DrawResult, error) {
	return fetchKJOnce()
}

// StartDrawMonitor 轮询 kj.json，新开奖写入 channel（不做 Telegram 发送）。
func StartDrawMonitor(initialQihao int) <-chan *DrawResult {
	ch := make(chan *DrawResult, 4)
	go pollKJLoop(initialQihao, ch)
	log.Printf("[开奖] 监控启动 (间隔: %v)", pollInterval)
	return ch
}

func pollKJLoop(lastQ int, ch chan<- *DrawResult) {
	for {
		start := time.Now()
		if result, err := fetchKJOnce(); err == nil && result.Qihao > lastQ {
			lastQ = result.Qihao
			ch <- result
		}
		if rem := pollInterval - time.Since(start); rem > 0 {
			time.Sleep(rem)
		}
	}
}

func fetchKJOnce() (*DrawResult, error) {
	data, err := httpGet(pc28Client, kjURL)
	if err != nil {
		return nil, err
	}
	return parseKJJSON(data)
}

func fetchPC28Map(url string) (map[string]int, error) {
	body, err := httpGet(pc28Client, url)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Data map[string]int `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if len(raw.Data) == 0 {
		return nil, fmt.Errorf("data empty")
	}
	return raw.Data, nil
}

func parseKJJSON(data []byte) (*DrawResult, error) {
	var resp struct {
		Data []struct {
			Nbr         string `json:"nbr"`
			Number      string `json:"number"`
			Num         string `json:"num"`
			Combination string `json:"combination"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("kj data empty")
	}
	item := resp.Data[0]

	qihao, err := strconv.Atoi(strings.TrimSpace(item.Nbr))
	if err != nil {
		return nil, fmt.Errorf("kj nbr invalid: %q", item.Nbr)
	}
	nums, err := parseDrawNumber(item.Number)
	if err != nil {
		return nil, err
	}
	sum, err := strconv.Atoi(strings.TrimSpace(item.Num))
	if err != nil {
		return nil, fmt.Errorf("kj num invalid: %q", item.Num)
	}
	if nums[0]+nums[1]+nums[2] != sum {
		return nil, fmt.Errorf("kj sum mismatch: %s=%d", item.Number, sum)
	}
	size, parity, err := parseCombination(item.Combination)
	if err != nil {
		return nil, err
	}

	return &DrawResult{
		Qihao:      qihao,
		Numbers:    nums,
		Sum:        sum,
		SizeType:   size,
		ParityType: parity,
		Pattern:    classifyPattern(nums[0], nums[1], nums[2]),
	}, nil
}

func parseDrawNumber(raw string) ([3]int, error) {
	parts := strings.Split(raw, "+")
	if len(parts) != 3 {
		return [3]int{}, fmt.Errorf("开奖号格式无效: %s", raw)
	}
	var nums [3]int
	for i, part := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return [3]int{}, fmt.Errorf("开奖号数字无效: %s", raw)
		}
		nums[i] = n
	}
	return nums, nil
}

func parseCombination(raw string) (size, parity string, err error) {
	r := []rune(strings.TrimSpace(raw))
	if len(r) != 2 {
		return "", "", fmt.Errorf("combination 无效: %q", raw)
	}
	size, parity = string(r[0]), string(r[1])
	if (size != "大" && size != "小") || (parity != "单" && parity != "双") {
		return "", "", fmt.Errorf("combination 无效: %q", raw)
	}
	return size, parity, nil
}

func classifyPattern(n1, n2, n3 int) string {
	if n1 == n2 && n2 == n3 {
		return "豹子"
	}
	if n1 == n2 || n1 == n3 || n2 == n3 {
		return "对子"
	}
	mn, mx := n1, n1
	if n2 < mn {
		mn = n2
	}
	if n3 < mn {
		mn = n3
	}
	if n2 > mx {
		mx = n2
	}
	if n3 > mx {
		mx = n3
	}
	if mx-mn == 2 {
		return "顺子"
	}
	return "杂六"
}
