package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	configPath := flag.String("config", "", "指定 config.toml 路径")
	showConsole := flag.Bool("console", false, "是否挂载控制台黑框打印调试日志")
	debug := flag.Bool("debug", false, "是否开启 debug 模式")
	silent := flag.Bool("silent", false, "是否静默启动 (仅托盘常驻，不自动弹窗)")
	flag.Parse()

	// 1. 根据参数判断是否挂载控制台窗口 (默认无黑框)
	ui.EnsureConsole(*showConsole || *debug)

	log.Println("==================================================")
	log.Printf("🎮 ImLagGoGoGo v%s - CS2 死后全自动狡辩系统 启动中...", config.Version)
	log.Println("==================================================")

	// 2. 加载配置
	cfg, err := config.LoadOrCreate(*configPath)
	if err != nil {
		log.Fatalf("❌ 加载配置失败: %v", err)
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

	// 6. 启动 GSI HTTP 服务端 (在 [gsi_port, max_port] 自动寻找可用端口)
	gsiSrv := gsi.NewServer(cfg.Network.GSIPort, cfg.Network.MaxPort)
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
	}

	// 7. 启动词库文件热监听 (fsnotify)
	if cfg.Corpus.WatchFile {
		startCorpusWatcher(cfg.Corpus.FilePath, cm)
	}

	// 8. 启动本地 CS2 Panorama 风格 UI 界面
	uiSrv := ui.NewUIServer(cfg, cm, gsiSrv, eng, drv)
	uiPort, err := uiSrv.Start()
	if err != nil {
		log.Printf("⚠️ UI 管理服务启动失败: %v", err)
	} else {
		uiURL := fmt.Sprintf("http://127.0.0.1:%d", uiPort)
		log.Printf("🌐 UI 控制面板已就绪: %s", uiURL)

		if !*silent {
			_ = ui.OpenNativeWindow(uiURL)
		}
	}

	// 9. 启动系统托盘图标
	go func() {
		systray.Run(func() {
			onSystrayReady(cfg, cm, eng, uiPort)
		}, func() {
			log.Println("托盘退出")
		})
	}()

	// 优雅终止信号捕获
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("🛑 正在优雅退出...")
	_ = gsiSrv.Stop()
	systray.Quit()
}

func startCorpusWatcher(filePath string, cm *corpus.CorpusManager) {
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
		systray.Quit()
		os.Exit(0)
	})
}
