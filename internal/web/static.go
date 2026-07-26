// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 hchw

package web

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed dist
var dist embed.FS

func ServeStatic(r *gin.Engine) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return
	}
	r.StaticFS("/ui", http.FS(sub))
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/ui/")
	})
}
