// Copyright 2018 Drone.IO Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package web

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"go.codycody31.dev/squad-aegis/web"
)

var indexHTML []byte

type prefixFS struct {
	fs     http.FileSystem
	prefix string
}

func (f *prefixFS) Open(name string) (http.File, error) {
	return f.fs.Open(strings.TrimPrefix(name, f.prefix))
}

// New returns a gin engine to serve the web frontend.
func New(db *sql.DB) (*gin.Engine, error) {
	e := gin.New()
	var err error
	indexHTML, err = parseIndex()
	if err != nil {
		return nil, err
	}

	// Add database to context
	e.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	// rootPath := server.Config.Server.RootPath
	rootPath := "/"
	httpFS, err := web.HTTPFS()
	if err != nil {
		return nil, err
	}
	f := &prefixFS{httpFS, rootPath}

	// Serve favicon files
	e.GET(rootPath+"/favicon.svg", serveFile(f))
	e.GET(rootPath+"/favicon.ico", serveFile(f))

	// Keep existing routes
	e.GET(rootPath+"/assets/*filepath", serveFile(f))

	e.NoRoute(handleIndex)

	return e, nil
}

func serveFile(f *prefixFS) func(ctx *gin.Context) {
	return func(ctx *gin.Context) {
		file, err := f.Open(ctx.Request.URL.Path)
		if err != nil {
			code := http.StatusInternalServerError
			if errors.Is(err, fs.ErrNotExist) {
				code = http.StatusNotFound
			} else if errors.Is(err, fs.ErrPermission) {
				code = http.StatusForbidden
			}
			ctx.Status(code)
			return
		}
		data, err := io.ReadAll(file)
		if err != nil {
			ctx.Status(http.StatusInternalServerError)
			return
		}
		var mime string
		switch {
		case strings.HasSuffix(ctx.Request.URL.Path, ".js"):
			mime = "text/javascript"
		case strings.HasSuffix(ctx.Request.URL.Path, ".css"):
			mime = "text/css"
		case strings.HasSuffix(ctx.Request.URL.Path, ".png"):
			mime = "image/png"
		case strings.HasSuffix(ctx.Request.URL.Path, ".svg"):
			mime = "image/svg+xml"
		case strings.HasSuffix(ctx.Request.URL.Path, ".ico"):
			mime = "image/x-icon"
		case strings.HasSuffix(ctx.Request.URL.Path, ".webmanifest"):
			mime = "application/manifest+json"
		case strings.HasSuffix(ctx.Request.URL.Path, ".woff"):
			mime = "font/woff"
		case strings.HasSuffix(ctx.Request.URL.Path, ".woff2"):
			mime = "font/woff2"
		case strings.HasSuffix(ctx.Request.URL.Path, ".map"):
			mime = "application/json"
		}
		ctx.Status(http.StatusOK)
		ctx.Writer.Header().Set("Cache-Control", "public, max-age=31536000")
		ctx.Writer.Header().Del("Expires")
		if mime != "" {
			ctx.Writer.Header().Set("Content-Type", mime)
		}
		if _, err := ctx.Writer.Write(replaceBytes(data)); err != nil {
			log.Error().Err(err).Msgf("cannot write %s", ctx.Request.URL.Path)
		}
	}
}

func handleIndex(c *gin.Context) {
	rw := c.Writer
	rw.Header().Set("Cache-Control", "no-cache")
	rw.Header().Set("Content-Type", "text/html; charset=UTF-8")
	rw.WriteHeader(http.StatusOK)
	if _, err := rw.Write(indexHTML); err != nil {
		log.Error().Err(err).Msg("cannot write index.html")
	}
}

func loadFile(path string) ([]byte, error) {
	data, err := web.Lookup(path)
	if err != nil {
		return nil, err
	}
	return replaceBytes(data), nil
}

func replaceBytes(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("/BASE_PATH"), []byte("/"))
}

func parseIndex() ([]byte, error) {
	data, err := loadFile("index.html")
	if err != nil {
		return nil, fmt.Errorf("cannot find index.html: %w", err)
	}
	return data, nil
}
