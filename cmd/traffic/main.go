package main

import (
	"log/slog"
	"os"
	"traffic/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		slog.Error("traffic stopped", "error", err)
		os.Exit(1)
	}
}
