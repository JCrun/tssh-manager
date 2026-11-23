package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

// version is set at build time: -ldflags "-X main.version=v0.1.0"
var version = "dev"

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "tssh-manager",
		Width:     1180,
		Height:    760,
		MinWidth:  860,
		MinHeight: 520,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 24, G: 24, B: 27, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		Linux: &linux.Options{
			Icon: icon,
			// Must match StartupWMClass in the .desktop file so the dock groups the window.
			ProgramName: "tssh-manager",
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
