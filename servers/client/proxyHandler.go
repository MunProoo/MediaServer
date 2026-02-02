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
			// getM3U8Info(c)
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
	} else if len(reqSlice) == 3 {
		switch reqSlice[2] {
		case "html":
			serveHTMLProxy(c)
		case "resource":
			serveResourceProxy(c)
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

func getM3U8List(c *gin.Context) {
	msclient := getMediaServerClient()
	reqQuery := c.Request.URL.Query()

	streamID := reqQuery.Get("streamID")
	date := reqQuery.Get("date")

	// 쿼리 파라미터 구성
	queryParams := url.Values{}
	if date != "" {
		queryParams.Set("date", date)
	}
	if streamID != "" {
		queryParams.Set("streamID", streamID)
	}

	// request 메서드에 쿼리 파라미터 전달
	resp, err := msclient.request("GET", "/stream/recording/list", nil, queryParams)
	if err != nil {
		log.Printf("Request failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "failed to api request",
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		dump2, _ := httputil.DumpResponse(resp, true)
		log.Printf("Response error (status %d): %s", resp.StatusCode, string(dump2))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "media server returned error",
		})
		return
	}

	// 응답을 그대로 클라이언트에 전달
	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

// serveM3U8 M3U8 파일 서빙 (TODO: 구현 필요)
func serveM3U8(c *gin.Context) {
	msclient := getMediaServerClient()
	reqQuery := c.Request.URL.Query()

	streamID := reqQuery.Get("StreamID")
	channelID := reqQuery.Get("ChannelID")
	fileName := reqQuery.Get("Filename")

	// 쿼리 파라미터 구성
	queryParams := url.Values{}
	if streamID != "" {
		queryParams.Set("streamID", streamID)
	}
	if channelID != "" {
		queryParams.Set("channel", channelID)
	}
	if fileName != "" {
		queryParams.Set("m3u8File", fileName)
	}

	// request 메서드에 쿼리 파라미터 전달
	resp, err := msclient.request("GET", "/stream/recording/m3u8", nil, queryParams)
	if err != nil {
		log.Printf("Request failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "failed to api request",
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		dump2, _ := httputil.DumpResponse(resp, true)
		log.Printf("Response error (status %d): %s", resp.StatusCode, string(dump2))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "media server returned error",
		})
		return
	}

	// m3u8 읽기
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("failed to read m3u8 content: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "failed to read m3u8 content",
		})
		return
	}

	// m3u8 내용 수정 (TS 파일 경로를 프록시 경로로 수정)
	newM3U8 := replaceM3U8(queryParams, string(body))
	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.Status(http.StatusOK)
	c.Writer.Write([]byte(newM3U8))
}

// serveTS TS 파일 서빙 (TODO: 구현 필요)
func serveTS(c *gin.Context) {
	reqQuery := c.Request.URL.Query()
	token := reqQuery.Get("token")
	core2UrlMapMutex.Lock()
	segURLStr, exist := core2UrlMap[token]
	core2UrlMapMutex.Unlock()
	if !exist {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "token not found",
		})
		return
	}

	msClient := getMediaServerClient()
	resp, err := msClient.request("GET", segURLStr, nil)
	if err != nil {
		log.Printf("Request failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "failed to api request",
		})
		return
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		dump2, _ := httputil.DumpResponse(resp, true)
		log.Printf("Response error (status %d): %s", resp.StatusCode, string(dump2))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "media server returned error",
		})
		return
	}

	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	c.Header("Cache-Control", resp.Header.Get("Cache-Control"))
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body) // TS 파일 스트리밍
}

// serveKey Key 파일 서빙 (TODO: 구현 필요)
func serveKey(c *gin.Context) {
	reqQuery := c.Request.URL.Query()
	token := reqQuery.Get("token")
	core2UrlMapMutex.Lock()
	keyURI, exist := core2UrlMap[token]
	core2UrlMapMutex.Unlock()

	if !exist {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "token not found",
		})
		return
	}

	msClient := getMediaServerClient()
	resp, err := msClient.request("GET", keyURI, nil)
	if err != nil {
		log.Printf("Request failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "failed to api request",
		})
		return
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		dump2, _ := httputil.DumpResponse(resp, true)
		log.Printf("Response error (status %d): %s", resp.StatusCode, string(dump2))
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "media server returned error",
		})
		return
	}

	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	c.Header("Cache-Control", resp.Header.Get("Cache-Control"))
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body) // Key 파일 전달
}

// HTML 프록시 서빙
func serveHTMLProxy(c *gin.Context) {
	reqQuery := c.Request.URL.Query()
	targetURL := reqQuery.Get("url")
	if targetURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "url is required",
		})
		return
	}

	// HTTP 클라이언트로 원본 페이지 가져오기
	msClient := getMediaServerClient()
	resp, err := msClient.request("GET", targetURL, nil)
	if err != nil {
		log.Printf("Request failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "failed to api request",
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("target server returned error: %s", resp.Status)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "target server returned error",
		})
		return
	}

	contentType := resp.Header.Get("Content-Type")

	// HTML인 경우 리소스 경로 변경
	if strings.Contains(contentType, "text/html") {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("failed to read response body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to read response body",
			})
			return
		}

		// HTML 파싱 및 리소스 경로 변경
		// HTML 파싱, 스크립트 주입 및 리소스 경로 변경
		modifiedHTML, err := injectProxyScriptAndRewriteHTML(string(bodyBytes), c)
		if err != nil {
			log.Printf("failed to rewrite HTML: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to rewrite HTML",
			})
			return
		}

		c.Header("Content-Type", contentType)
		c.Status(http.StatusOK)
		c.Writer.Write([]byte(modifiedHTML))
	} else {
		// HTML이 아닌 경우 (CSS, JS, 이미지 등) 그대로 전달
		for key, values := range resp.Header {
			for _, value := range values {
				c.Header(key, value)
			}
		}
		c.Status(resp.StatusCode)
		io.Copy(c.Writer, resp.Body)
	}
}

func serveResourceProxy(c *gin.Context) {
	reqQuery := c.Request.URL.Query()
	resourceURL := reqQuery.Get("url")
	if resourceURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "url is required",
		})
		return
	}

	msClient := getMediaServerClient()
	resp, err := msClient.request("GET", resourceURL, nil)
	if err != nil {
		log.Printf("Request failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "failed to api request",
		})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("target server returned error: %s", resp.Status)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "target server returned error",
		})
		return
	}

	contentType := resp.Header.Get("Content-Type")

	// CSS 파일만 URL 변경 필요 (Javascript는 클라이언트 측에서 처리)
	if strings.Contains(contentType, "text/css") {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("failed to read response body: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "failed to read response body",
			})
			return
		}

		modifiedCSS := rewriteCSSURLs(string(bodyBytes), c)
		c.Header("Content-Type", contentType)
		c.Status(http.StatusOK)
		c.Writer.Write([]byte(modifiedCSS))
		return // CSS면 변경 후 return
	}

	for key, values := range resp.Header {
		for _, value := range values {
			c.Writer.Header().Add(key, value)
		}
	}

	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
	return
}
