package server

import (
	"bytes"
	"image/png"
	"net/http"
	"strconv"

	"github.com/MuhammadSaim/goavatar"
	"github.com/gin-gonic/gin"
)

func (s *Server) GetAvatar(c *gin.Context) {
	username := c.Query("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username is required"})
		return
	}

	widthStr := c.Query("width")
	heightStr := c.Query("height")
	gridsizeStr := c.Query("gridsize")

	if widthStr == "" {
		widthStr = "256"
	}
	if heightStr == "" {
		heightStr = "256"
	}
	if gridsizeStr == "" {
		gridsizeStr = "8"
	}

	width, err := strconv.Atoi(widthStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid width"})
		return
	}
	height, err := strconv.Atoi(heightStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid height"})
		return
	}
	gridsize, err := strconv.Atoi(gridsizeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid gridsize"})
		return
	}

	if width < 100 || width > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid width"})
		return
	}
	if height < 100 || height > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid height"})
		return
	}
	if gridsize < 1 || gridsize > 32 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid gridsize"})
		return
	}

	options := goavatar.Options{
		Width:    width,
		Height:   height,
		GridSize: gridsize,
	}
	avatar := goavatar.Make(username, options)

	buf := new(bytes.Buffer)
	err = png.Encode(buf, avatar)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode image"})
		return
	}

	c.Data(http.StatusOK, "image/png", buf.Bytes())
}
