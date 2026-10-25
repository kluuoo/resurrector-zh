package main

import (
	"embed"
	"fmt"
	"os"
	"runtime/debug" // +++ 新增：用于捕获堆栈
    
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
    // ... 保留你原本所有的 import ...
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// +++ 新增：装甲启动，无论发生什么崩溃，都会写进 crash.log
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("致命崩溃: %v\n\n堆栈信息:\n%s", r, debug.Stack())
			os.WriteFile("crash.log", []byte(errMsg), 0644)
		}
	}()

	// ---- 下面全部是原作者的原始代码，一个字都不要改！----
	app := NewApp() 
    
    // (如果原作者是 app := NewApp(ctx)，那也原样保留)

	err := wails.Run(&options.App{
        // (保留原作者所有的选项，比如 Title, Width, OnStartup, Tray 等)
	})

	if err != nil {
		os.WriteFile("startup_error.log", []byte(err.Error()), 0644)
		fmt.Println("启动失败:", err.Error())
	}
}
