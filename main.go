package main

import (
	"io"
	"kan28/config"
	"kan28/service"
	"log"
	"os"
)

func main() {
	logFile, err := os.OpenFile("bobao.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(io.MultiWriter(os.Stdout, logFile))
		defer logFile.Close()
	}

	log.Println("=== 加拿大28开奖播报机器人启动 ===")

	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	tgService, err := service.NewTelegramService(cfg)
	if err != nil {
		log.Fatalf("Telegram服务初始化失败: %v", err)
	}

	if err := service.InitDrawImage(service.DefaultDrawBg, service.DefaultDrawFont); err != nil {
		log.Printf("[图片] 渲染器初始化失败，降级纯文字: %v", err)
	}

	service.InitPC28()
	service.InitYilouCache()

	var initialQ int
	if r, err := service.FetchLatestDraw(); err == nil {
		initialQ = r.Qihao
		log.Printf("当前最新: %d期，等待新开奖...", initialQ)
	}

	drawCh := service.StartDrawMonitor(initialQ)
	go tgService.StartUpdateHandler()
	log.Println("服务已启动")

	lastQ := initialQ
	for r := range drawCh {
		if r.Qihao <= lastQ {
			continue
		}
		log.Printf("[NEW] %d期 %d+%d+%d=%d",
			r.Qihao, r.Numbers[0], r.Numbers[1], r.Numbers[2], r.Sum)

		tgService.Broadcast(r)
		service.GetYilouCache().RefreshNow()
		lastQ = r.Qihao
	}
}
