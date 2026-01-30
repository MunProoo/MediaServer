package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

func proxyHandler(c *gin.Context) {
	method := c.Request.Method

	switch method {
	case "GET":
		handleGet(c)
	case "POST":
		handlePost(c)
	case "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD":
		// 다른 메서드도 필요시 처리 가능
		handlePost(c) // 기본적으로 POST와 동일하게 처리
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Method %s not supported", method),
		})
	}
}

func handleGet(c *gin.Context) {
	reqSlice := strings.Split(c.Request.URL.Path, "/")

	if len(reqSlice) == 4 && reqSlice[2] == "core" {
		switch reqSlice[3] {
		case "m3u8Info":
			getM3U8Info(c)
		case "ts":
			serveTS(c)
		case "key":
			serveKey(c)
		case "m3u8List":
			getM3U8List(c)
		case "m3u8":
			serveM3U8(c)
		default:
			c.JSON(http.StatusOK, gin.H{
				"status":  "error",
				"message": fmt.Sprintf("Method %s not supported", reqSlice[3]),
			})
		}
	}
}

func handlePost(c *gin.Context) {
	reqSlice := strings.Split(c.Request.URL.Path, "/")

	if len(reqSlice) == 3 {
		if reqSlice[2] == "signalling" {
			handleSignalling(c)
		} else if reqSlice[2] == "resource" {
			// serveFormSubmit(c) // setting 할거면 필요
		}
	}
}

func handleSignalling(c *gin.Context) {
	query := c.Request.URL.Query()

	streamID := query.Get("streamID")
	channelID := query.Get("channelID")
	sdpOffer := c.PostForm("data")

	formData := url.Values{}
	formData.Set("data", sdpOffer)

	baseURL := fmt.Sprintf("%s/stream/%s/channel/%s/webrtc", mediaServerClient.baseURL, streamID, channelID)

	// Media 서버에서 m3u8 가져오기
	apiClient := globalClientPool.Get().(*http.Client)
	resp, err := apiClient.PostForm(baseURL, formData)
	if err != nil {
		log.Printf("failed to api request: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "failed to api request",
		})
		return
	}
	defer resp.Body.Close()
	defer globalClientPool.Put(apiClient)

	if resp.StatusCode != http.StatusOK {
		dump2, _ := httputil.DumpResponse(resp, true) // true = body 포함
		log.Printf("media Server returned error: %s", string(dump2))
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "media Server returned error",
		})
		return
	}

	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	c.Status(http.StatusOK)

	// key file 전달
	io.Copy(c.Writer, resp.Body)
}
