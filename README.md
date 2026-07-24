# Telegram PC28 开奖播报

面向 Telegram 频道的加拿大 28（PC28）开奖自动播报机器人。

新开奖到达后，自动向频道发送图文消息；可一键查看遗漏、今日统计，并跳转同和值历史开奖。数据来自 [pc28.help](https://pc28.help) 公开接口。

**作者**：[Telegram @yuuu](https://t.me/yuuu)

---

## 功能特性

| 能力 | 说明 |
| --- | --- |
| 开奖播报 | 新开奖自动推送到频道，每期不漏、不重复 |
| 图文消息 | 开奖图 + 文字说明；图片异常时仍以文字完整播报 |
| 遗漏查询 | 点击按钮，弹窗查看号码与玩法遗漏 |
| 今日统计 | 点击按钮，弹窗查看当日大小单双等统计 |
| 和值对应 | 「对应」一键跳到上一次相同和值的开奖消息 |

---

## 系统架构

```text
                 ┌─────────────────────────┐
                 │   pc28.help JSON API    │
                 │  kj / yl / yk           │
                 └───────────┬─────────────┘
                             │ HTTP 轮询
                             ▼
┌──────────────┐     ┌───────────────┐     ┌────────────────────┐
│   main.go    │────▶│  draw monitor │────▶│ drawCh (新期号)     │
│  启动编排     │     │  500ms 轮询    │     └─────────┬──────────┘
└──────────────┘     └───────────────┘               │
                                                     ▼ lastQ 去重
                                            ┌────────────────────┐
                                            │ TelegramService    │
                                            │ Broadcast → queue  │
                                            │ sendWorker 串行发送 │
                                            └─────────┬──────────┘
                                                      │
                         ┌────────────────────────────┼────────────────────────────┐
                         ▼                            ▼                            ▼
                  开奖图渲染/上传              sendMessage / sendPhoto         异步清旧按钮
                  失败→纯文字降级              成功后写 last_msg.json           YilouCache 刷新
```

### 数据源

| 接口 | 用途 |
| --- | --- |
| `https://pc28.help/api/kj.json` | 最新开奖（期号、号码、和值、组合） |
| `https://pc28.help/api/yl.json` | 遗漏数据 |
| `https://pc28.help/api/yk.json` | 今日统计 |

本项目**不依赖**数据库或 Redis。

---

## 技术栈

- **语言**：Go 1.25+
- **Telegram**：[go-telegram-bot-api/v5](https://github.com/go-telegram-bot-api/telegram-bot-api)
- **图片**：`golang.org/x/image` + FreeType 字体渲染
- **配置**：`.env`（`godotenv`）
- **产物名**：`bobao`（`make build`）

---

## 目录结构

```text
.
├── main.go                 # 进程入口：配置、初始化、主循环
├── Makefile                # build / linux / test
├── go.mod / go.sum
├── .env.example            # 环境变量模板（勿提交真实密钥）
├── assets/
│   ├── draw_bg.jpg         # 开奖图底图
│   └── AlimamaShuHeiTi-Bold.ttf
├── config/
│   └── config.go           # 环境变量加载与校验
└── service/
    ├── http.go             # DNS pin、通用 HTTP GET
    ├── draw.go             # 开奖轮询与 JSON 解析
    ├── stats.go            # 遗漏缓存、统计弹窗
    ├── telegram.go         # 播报队列、发送、按钮回调
    ├── draw_image.go       # 开奖图渲染
    └── draw_test.go        # 解析与键盘单测
```

---

## 快速开始

### 1. 环境要求

- Go 1.25 或更高版本
- 可用的 Telegram Bot Token
- Bot 已加入目标频道，并具备发消息权限

### 2. 配置

```bash
cp .env.example .env
```

编辑 `.env`：

```env
TELEGRAM_BOT_TOKEN=123456:ABC...
TELEGRAM_CHANNEL_ID=-100xxxxxxxxxx
TELEGRAM_CHANNEL_USERNAME=your_channel
```

| 变量 | 必填 | 说明 |
| --- | --- | --- |
| `TELEGRAM_BOT_TOKEN` | 是 | BotFather 发放的 Token |
| `TELEGRAM_CHANNEL_ID` | 是 | 频道数字 ID（通常为负数） |
| `TELEGRAM_CHANNEL_USERNAME` | 建议 | 不带 `@`；用于「对应」深链接 |

### 3. 编译与运行

```bash
# 本机产物
make build
./bobao

# Linux amd64 交叉编译
make linux

# 测试
make test
```

运行目录需能读取 `assets/` 与写入 `bobao.log`、`last_msg.json`（建议在项目根目录启动）。

---

## 运行时行为

1. **启动**：加载配置 → 初始化 Telegram / 开奖图 / pc28 客户端 / 遗漏缓存 → 拉取当前最新期号作为 `lastQ`。
2. **检测**：独立协程每 500ms 请求 `kj.json`；仅当期号大于 `lastQ` 时写入 `drawCh`。
3. **播报**：主循环去重后调用 `Broadcast`（非阻塞入队）；`sendWorker` 串行发送。
4. **降级**：开奖图渲染或上传失败时回退纯文字，不中断播报。
5. **状态**：拿到 `message_id` 后才更新内存并原子写入 `last_msg.json`。
6. **遗漏**：进程内缓存，默认 3 秒刷新；新开奖后异步 `RefreshNow()`；拉取失败保留上次有效数据。
7. **键盘**：仅功能按钮——遗漏 / 统计 / 对应；发送新消息后可异步清除上一条消息按钮。

---

## 部署建议

1. 服务器安装 Go，或直接上传 `make linux` 得到的 `bobao-linux`。
2. 同步代码与 `assets/`，配置运行目录下的 `.env`（不要把真实 Token 写入仓库）。
3. 使用 systemd、Supervisor 或面板进程管理守护 `./bobao`。
4. 部署后由运维在面板重启进程；更新代码后重复：拉代码 → 构建 → 人工重启 → 观察日志。

### 健康检查

- 日志出现 `当前最新: …期，等待新开奖...` 与 `服务已启动`
- 新开奖后出现 `[NEW]` 与 `[OK] … TG播报完成`
- 频道消息带遗漏 / 统计 / 对应按钮，弹窗数据正常

### 回滚

保留上一版可执行文件与同版本 `assets/`；异常时切回旧二进制并人工重启即可。`last_msg.json` 可继续沿用，避免对应链接断裂。

---

## 安全与合规

- **禁止**将 Bot Token、频道私钥或真实 `.env` 提交到 Git。
- 仓库仅包含 `.env.example` 作为变量清单。
- 生产环境变量建议通过面板 / 密钥管理系统注入。
- 本程序只读公开开奖 API，并向你有权限的 Telegram 频道发送消息；请遵守 Telegram Bot 使用条款与当地法规。

---

## 许可证

本项目代码由作者维护，仅供学习与自有频道运营使用。二次分发或商用请事先联系作者。

---

## 作者

- Telegram：[@yuuu](https://t.me/yuuu)
- 仓库维护：开源公开，Issue / 建议可通过 Telegram 联系
