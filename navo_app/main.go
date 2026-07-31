package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	startHidden := false
	for _, argument := range os.Args[1:] {
		if argument == "--start-hidden" {
			startHidden = true
			break
		}
	}
	webviewDataPath := filepath.Join(os.Getenv("LOCALAPPDATA"), "Navo", "WebView2")
	if root := os.Getenv("LOCALAPPDATA"); root == "" {
		if cacheRoot, cacheErr := os.UserCacheDir(); cacheErr == nil {
			webviewDataPath = filepath.Join(cacheRoot, "Navo", "WebView2")
		}
	}

	app := NewApp()
	err := wails.Run(&options.App{
		Title:             "Navo",
		Width:             1180,
		Height:            760,
		MinWidth:          900,
		MinHeight:         620,
		StartHidden:       startHidden,
		HideWindowOnClose: false,
		BackgroundColour:  options.NewRGB(20, 16, 39),
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		Bind:      []interface{}{app},
		Windows: &windows.Options{
			Theme:               windows.SystemDefault,
			BackdropType:        windows.Mica,
			WindowClassName:     "NavoAppWindow",
			WebviewUserDataPath: webviewDataPath,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "start Navo UI:", err)
		os.Exit(1)
	}
}
