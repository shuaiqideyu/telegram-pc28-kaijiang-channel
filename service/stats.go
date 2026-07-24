package service

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// YilouCache 进程内遗漏缓存；读失败时保留上次有效数据。
type YilouCache struct {
	mu   sync.RWMutex
	data map[string]int
}

var globalYilouCache *YilouCache

func InitYilouCache() {
	globalYilouCache = &YilouCache{}
	globalYilouCache.update()
	go globalYilouCache.loop()
	log.Println("遗漏缓存已启动 (yl.json, 3s)")
}

func GetYilouCache() *YilouCache { return globalYilouCache }

func (yc *YilouCache) loop() {
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for range t.C {
		yc.update()
	}
}

func (yc *YilouCache) update() {
	m, err := fetchPC28Map(ylURL)
	if err != nil {
		return
	}
	yc.mu.Lock()
	defer yc.mu.Unlock()
	if mapsEqualInt(yc.data, m) {
		return
	}
	yc.data = m
	log.Println("遗漏数据缓存已更新")
}

func (yc *YilouCache) GetData() (map[string]int, error) {
	yc.mu.RLock()
	defer yc.mu.RUnlock()
	if yc.data == nil {
		return nil, fmt.Errorf("遗漏数据缓存未就绪")
	}
	out := make(map[string]int, len(yc.data))
	for k, v := range yc.data {
		out[k] = v
	}
	return out, nil
}

// RefreshNow 新开奖后异步刷新遗漏缓存。
func (yc *YilouCache) RefreshNow() {
	if yc == nil {
		return
	}
	go yc.update()
}

func FormatYilouMessage(m map[string]int) string {
	return fmt.Sprintf("🎲PC28 未开统计：\n0:%d期，27:%d期\n1:%d期，26:%d期\n2:%d期，25:%d期\n3:%d期，24:%d期\n4:%d期，23:%d期\n5:%d期，22:%d期\n13:%d期，14:%d期\n豹子:%d期，顺子:%d期\n极大:%d期，极小:%d期",
		m["00"], m["27"],
		m["01"], m["26"],
		m["02"], m["25"],
		m["03"], m["24"],
		m["04"], m["23"],
		m["05"], m["22"],
		m["13"], m["14"],
		m["豹子"], m["顺子"],
		m["极大"], m["极小"],
	)
}

func FormatTongjiMessage() (string, error) {
	m, err := fetchPC28Map(ykURL)
	if err != nil {
		return "", fmt.Errorf("获取统计失败: %v", err)
	}
	return fmt.Sprintf("📚今日统计：\n大：%d  小：%d\n单：%d  双：%d\n\n00：%d\n27：%d\n\n01：%d\n26：%d\n\n13：%d\n14：%d\n\n豹子：%d\n顺子：%d\n极大：%d\n极小：%d",
		m["大"], m["小"],
		m["单"], m["双"],
		m["00"], m["27"],
		m["01"], m["26"],
		m["13"], m["14"],
		m["豹子"], m["顺子"],
		m["极大"], m["极小"],
	), nil
}

func mapsEqualInt(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
