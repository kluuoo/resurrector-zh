package main

import (
	"embed"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 1. 日志重定向：把标准输出和错误输出写进文件
	logFile, _ := os.OpenFile("app_debug.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	os.Stdout = logFile
	os.Stderr = logFile

	// 2. 崩溃捕获：如果发生致命错误，把堆栈信息也写进去
	defer func() {
		if r := recover(); r != nil {
			logFile.WriteString(fmt.Sprintf("\n[致命崩溃] %v\n%s", r, debug.Stack()))
		}
		logFile.Close()
	}()

	// 3. 原封不动的启动逻辑
	app := NewApp()

	err := wails.Run(&options.App{
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		logFile.WriteString(fmt.Sprintf("\n[启动失败] %v", err))
	}
}
