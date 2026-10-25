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

// 这段代码会在程序启动前运行。如果发生崩溃，会把错误写进 log 文件里。
func init() {
	if r := recover(); r != nil {
		errMsg := fmt.Sprintf("崩溃原因: %v\n\n堆栈信息:\n%s", r, debug.Stack())
		os.WriteFile("crash_error.log", []byte(errMsg), 0644)
	}
}

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "Resurrector",
		Width:  800,
		Height: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		os.WriteFile("startup_error.log", []byte(err.Error()), 0644)
		fmt.Println("启动失败:", err.Error())
	}
}
