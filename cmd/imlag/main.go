package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"github.com/VincentZyuApps/ImLagGoGoGo/assets"
	"github.com/VincentZyuApps/ImLagGoGoGo/internal/config"
	"github.com/VincentZyuApps/ImLagGoGoGo/internal/corpus"
	"github.com/VincentZyuApps/ImLagGoGoGo/internal/driver"
	"github.com/VincentZyuApps/ImLagGoGoGo/internal/gsi"
	"github.com/VincentZyuApps/ImLagGoGoGo/internal/state"
	"github.com/VincentZyuApps/ImLagGoGoGo/internal/ui"
	"github.com/energye/systray"
	"github.com/fsnotify/fsnotify"
)

func main() {
	// 0. 初始化 Windows Job Object (确保进程树强绑定，防止 Edge 战术小窗口残留孤儿进程)
	ui.InitJobObject()

	configPath := flag.String("config", "", "指定 config.toml 路径")

	// 三维行为矩阵 Flags (默认 nil，用于判断命令行是否显式传入)
	flagConsole := flag.Bool("console", false, "是否挂载/打开原生控制台黑框 (默认 false)")
	flagBrowser := flag.Bool("browser", false, "是否调用系统默认浏览器打开 WebUI (默认 false)")
	flagWindow := flag.Bool("window", true, "是否打开独立精简战术 App 窗口 (默认 true)")

	debug := flag.Bool("debug", false, "是否开启 debug 模式 (等同于 --console)")
	silent := flag.Bool("silent", false, "是否静默启动 (等同于 --window=false --browser=false)")
	flag.Parse()

	// 2. 加载配置
	cfg, err := config.LoadOrCreate(*configPath)
	if err != nil {
		log.Fatalf("❌ 加载配置失败: %v", err)
	}

	// 3. 计算行为矩阵 (CLI Flags 优先级高于 config.toml)
	openConsole := cfg.UI.OpenConsole
	openBrowser := cfg.UI.OpenBrowser
	openWindow := cfg.UI.OpenWindow

	// 检查命令行是否显式指定了参数
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "console":
			openConsole = *flagConsole
		case "browser":
			openBrowser = *flagBrowser
		case "window":
			openWindow = *flagWindow
		case "debug":
			if *debug {
				openConsole = true
			}
		case "silent":
			if *silent {
				openWindow = false
				openBrowser = false
			}
		}
	})

	if *debug {
		openConsole = true
	}

	// 1. 根据行为矩阵判断是否挂载控制台窗口
	ui.EnsureConsole(openConsole)

	if logFile, err := os.OpenFile("imlag.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
		defer logFile.Close()
		if openConsole {
			log.SetOutput(io.MultiWriter(os.Stdout, logFile))
		} else {
			log.SetOutput(logFile)
		}
	}

	log.Println("==================================================")
	log.Printf("🎮 ImLagGoGoGo v%s - CS2 死后全自动狡辩系统 启动中...", config.Version)
	log.Println("==================================================")

	if !isElevated() {
		log.Println("💡 [权限提示] 当前以普通权限运行。若 CS2 以管理员权限运行，模拟按键可能会被 Windows UIPI 拦截；如遇按键无效请以管理员身份启动终端。")
	}

	// 3. 加载脱罪词库 (带智能转码与洗牌池)
	cm, err := corpus.NewCorpusManager(
		cfg.Corpus.FilePath,
		corpus.SelectionMode(cfg.Corpus.SelectionMode),
		cfg.Corpus.Encoding,
	)
	if err != nil {
		log.Fatalf("❌ 加载词库失败: %v", err)
	}
	totalWords, _ := cm.GetStats()
	log.Printf("📖 词库加载成功，共 %d 条脱罪借口", totalWords)

	// 4. 初始化跨平台驱动
	drv, err := driver.New(cfg.UI.CS2Path)
	if err != nil {
		log.Printf("⚠️ 驱动初始化警告: %v", err)
	}

	// 5. 初始化核心业务状态机
	eng := state.NewEngine(cfg, cm, drv)

	// 6. 启动 GSI HTTP 服务端 (在 [gsi_port, max_gsi_port] 自动寻找可用端口)
	gsiSrv := gsi.NewServer(cfg.Network.GSIPort, cfg.Network.MaxGSIPort)
	actualPort, err := gsiSrv.Start()
	if err != nil {
		log.Fatalf("❌ GSI 服务启动失败: %v", err)
	}
	log.Printf("📡 GSI 监听就绪，当前绑定端口: :%d", actualPort)

	// 绑定数据流到状态机
	gsiSrv.OnPayload(func(p *gsi.GSIPayload) {
		eng.ProcessPayload(p)
	})

	// 自动写入 CS2 的 GSI 配置文件
	if drv != nil {
		if err := drv.InstallGSIConfig(actualPort); err != nil {
			log.Printf("⚠️ 自动部署 GSI 配置提示: %v", err)
		} else {
			log.Println("✅ CS2 GSI 配置文件已全自动部署到游戏目录")
		}

		// 自动部署 CS2 原生 CFG 借口轮换链 (方案 A: 极速原生 say)
		allMsgs := cm.AllMessages()
		if len(allMsgs) > 0 {
			bindKeys := []string{cfg.Chat.CFGBindKey, cfg.Chat.CFGTeamBindKey}
			if err := drv.WriteSelectorCfg(allMsgs, bindKeys); err != nil {
				log.Printf("⚠️ 部署 CFG 轮换链提示: %v", err)
			} else {
				log.Printf("⚡ CS2 原生 CFG 借口链已部署 (方案 A 默认启用 | 全局键: [%s], 队内键: [%s])",
					strings.ToUpper(cfg.Chat.CFGBindKey), strings.ToUpper(cfg.Chat.CFGTeamBindKey))
				if drv.IsCS2Running() {
					log.Println("💡 [免重启提示] 检测到 CS2 正在运行中，在游戏控制台输入一次: exec autoexec 即可瞬间生效新 CFG！")
				}
			}
		}
	}

	// 7. 启动词库文件热监听 (fsnotify)
	if cfg.Corpus.WatchFile {
		startCorpusWatcher(cfg.Corpus.FilePath, cm, drv, cfg)
	}

	// 8. 启动本地 CS2 Panorama 风格 UI 界面
	uiSrv := ui.NewUIServer(cfg, cm, gsiSrv, eng, drv)
	uiPort, err := uiSrv.Start()
	if err != nil {
		log.Printf("⚠️ UI 管理服务启动失败: %v", err)
	} else {
		uiURL := fmt.Sprintf("http://127.0.0.1:%d", uiPort)
		log.Printf("🌐 UI 控制面板已就绪: %s", uiURL)

		// 根据展现形态矩阵呼出窗口
		if openWindow {
			log.Println("🖥️ 正在呼出独立 App 战术小窗口...")
			_ = ui.OpenNativeWindow(uiURL)
		}
		if openBrowser {
			log.Println("🌐 正在使用系统默认浏览器打开 WebUI...")
			_ = ui.OpenDefaultBrowser(uiURL)
		}
	}

	// 9. 启动系统托盘图标 (异步容错，即使无托盘环境亦不影响主对局引擎)
	go func() {
		systray.Run(func() {
			onSystrayReady(cfg, cm, eng, uiPort)
		}, func() {
			log.Println("托盘退出")
		})
	}()

	// 10. 统一多源优雅退出通道 (兼容控制台 Ctrl+C、系统信号与托盘动作)
	exitChan := make(chan string, 1)

	// 注册底层控制台钩子回调
	ui.RegisterShutdownHook(func(reason string) {
		select {
		case exitChan <- reason:
		default:
		}
	})

	// 注册 Go 运行时标准 OS 信号捕获
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-sigChan
		select {
		case exitChan <- fmt.Sprintf("系统信号 %v", s):
		default:
		}
	}()

	reason := <-exitChan
	log.Printf("🛑 正在停止 ImLagGoGoGo 服务 (触发原因: %s)...", reason)
	ui.CloseNativeWindow()
	_ = gsiSrv.Stop()
	systray.Quit()
	log.Println("👋 狡辩系统已安全退出，再见！")
	os.Exit(0)
}

func startCorpusWatcher(filePath string, cm *corpus.CorpusManager, drv driver.Driver, cfg *config.Config) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}

	_ = watcher.Add(filePath)

	go func() {
		defer watcher.Close()
		var lastReload time.Time
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {
					// 200ms 防抖
					if time.Since(lastReload) > 200*time.Millisecond {
						lastReload = time.Now()
						_ = cm.Reload()
						total, _ := cm.GetStats()
						log.Printf("🔄 词库已自动热重载，当前借口数: %d", total)
						// 同步热更新 CS2 CFG 轮换链
						if drv != nil {
							allMsgs := cm.AllMessages()
							if len(allMsgs) > 0 {
								bindKeys := []string{cfg.Chat.CFGBindKey, cfg.Chat.CFGTeamBindKey}
								_ = drv.WriteSelectorCfg(allMsgs, bindKeys)
							}
						}
					}
				}
			case <-watcher.Errors:
				return
			}
		}
	}()
}

func onSystrayReady(cfg *config.Config, cm *corpus.CorpusManager, eng *state.Engine, uiPort int) {
	systray.SetTitle("ImLagGoGoGo")
	systray.SetTooltip("ImLagGoGoGo - CS2 死后全自动狡辩系统")

	// 使用内嵌资产做基础
	if len(assets.DefaultConfigTOML) > 0 {
		// 图标占位
	}

	mStatus := systray.AddMenuItem("🟢 状态: 已启用 (点击暂停)", "点击切换暂停/启用")
	systray.AddSeparator()
	mOpenUI := systray.AddMenuItem("🖥️ 打开控制面板", "打开 CS2 战术仪表盘")
	mReload := systray.AddMenuItem("🔄 重新加载词库", "重载 Messages.txt")
	mTest := systray.AddMenuItem("🎯 3秒模拟测试", "测试发送一句借口")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("❌ 退出程序", "彻底退出")

	mStatus.Click(func() {
		paused := eng.TogglePause()
		if paused {
			mStatus.SetTitle("🔴 状态: 已暂停 (点击恢复)")
		} else {
			mStatus.SetTitle("🟢 状态: 已启用 (点击暂停)")
		}
	})

	mOpenUI.Click(func() {
		uiURL := fmt.Sprintf("http://127.0.0.1:%d", uiPort)
		_ = ui.OpenNativeWindow(uiURL)
	})

	mReload.Click(func() {
		_ = cm.Reload()
	})

	mTest.Click(func() {
		time.Sleep(3 * time.Second)
		_, _ = eng.ManualTrigger()
	})

	mQuit.Click(func() {
		ui.TriggerShutdown("用户通过系统托盘退出")
	})
}

func isElevated() bool {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}
