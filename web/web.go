package web

import (
	"embed"
	"net/http"
)

//go:embed index.html style.css app.js
var Assets embed.FS

func Handler() http.Handler {
	return http.FileServer(http.FS(Assets))
}
