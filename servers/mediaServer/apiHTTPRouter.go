package main

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"log"

	"github.com/gin-gonic/gin"
)

// Message resp struct
type Message struct {
	Status  int         `json:"status"`
	Payload interface{} `json:"payload"`
}

// HTTPAPIServer start http server routes
func HTTPAPIServer() {
	//Set HTTP API mode
	log.Printf("[INFO] [http_server] [RTSPServer] [Start] Server HTTP start")
	var public *gin.Engine
	if !Storage.ServerHTTPDebug() {
		gin.SetMode(gin.ReleaseMode)
		public = gin.New()
	} else {
		gin.SetMode(gin.DebugMode)
		public = gin.Default()
	}

	public.Use(CrossOrigin())
	//Add private login password protect methods
	privat := public.Group("/")
	if Storage.ServerHTTPLogin() != "" && Storage.ServerHTTPPassword() != "" {
		privat.Use(gin.BasicAuth(gin.Accounts{Storage.ServerHTTPLogin(): Storage.ServerHTTPPassword()}))
	}

	/*
		Static HTML Files Demo Mode
	*/

	if Storage.ServerHTTPDemo() {
		exePath, _ := os.Executable()
		exeDir := filepath.Dir(exePath)

		httpdir := filepath.Join(exeDir, Storage.ServerHTTPDir())
		public.LoadHTMLGlob(httpdir + "/templates/*")
		public.GET("/", HTTPAPIServerIndex)
		public.GET("/pages/stream/list", HTTPAPIStreamList)
		public.GET("/pages/stream/add", HTTPAPIAddStream)
		public.GET("/pages/stream/edit/:uuid", HTTPAPIEditStream)
		public.GET("/pages/player/hls/:uuid/:channel", HTTPAPIPlayHls)
		public.GET("/pages/player/mse/:uuid/:channel", HTTPAPIPlayMse)
		public.GET("/pages/player/webrtc/:uuid/:channel", HTTPAPIPlayWebrtc)
		public.GET("/pages/multiview", HTTPAPIMultiview)
		public.Any("/pages/multiview/full", HTTPAPIFullScreenMultiView)
		public.GET("/pages/documentation", HTTPAPIServerDocumentation)
		public.GET("/pages/player/all/:uuid/:channel", HTTPAPIPlayAll)
		public.GET("/pages/settings", HTTPAPIServerSettings)
		public.StaticFS("/static", http.Dir(httpdir+"/static"))

		// recordings 파일 동적 서빙 핸들러
		public.GET("/recordings/*filepath", HTTPAPIServerRecordingStatic)
		public.HEAD("/recordings/*filepath", HTTPAPIServerRecordingStatic)
	}

	/*
		Stream Control elements
	*/

	public.GET("/streams", HTTPAPIServerStreams)
	public.POST("/stream/:uuid/add", HTTPAPIServerStreamAdd)
	public.POST("/stream/:uuid/edit", HTTPAPIServerStreamEdit)
	public.GET("/stream/:uuid/delete", HTTPAPIServerStreamDelete)
	public.GET("/stream/:uuid/reload", HTTPAPIServerStreamReload)
	public.GET("/stream/:uuid/info", HTTPAPIServerStreamInfo)

	/*
		Streams Multi Control elements
	*/

	public.POST("/streams/multi/control/add", HTTPAPIServerStreamsMultiControlAdd)
	public.POST("/streams/multi/control/delete", HTTPAPIServerStreamsMultiControlDelete)

	/*
		Stream Channel elements
	*/

	public.POST("/stream/:uuid/channel/:channel/add", HTTPAPIServerStreamChannelAdd)
	public.POST("/stream/:uuid/channel/:channel/edit", HTTPAPIServerStreamChannelEdit)
	public.GET("/stream/:uuid/channel/:channel/delete", HTTPAPIServerStreamChannelDelete)
	public.GET("/stream/:uuid/channel/:channel/codec", HTTPAPIServerStreamChannelCodec)
	public.GET("/stream/:uuid/channel/:channel/reload", HTTPAPIServerStreamChannelReload)
	public.GET("/stream/:uuid/channel/:channel/info", HTTPAPIServerStreamChannelInfo)

	// server edit API 추가
	privat.POST("/server/edit", HTTPAPIServerEdit)
	public.POST("/pages/settings", HTTPAPIServerSettingsUpdate)
	public.GET("/api/maintenance/directories", HTTPAPIMaintenanceDirectories)

	/*
		Stream video elements
	*/
	//HLS
	public.GET("/stream/:uuid/channel/:channel/hls/live/index.m3u8", HTTPAPIServerStreamHLSM3U8)
	public.GET("/stream/:uuid/channel/:channel/hls/live/segment/:seq/file.ts", HTTPAPIServerStreamHLSTS)
	//HLS remote record
	//public.GET("/stream/:uuid/channel/:channel/hls/rr/:s/:e/index.m3u8", HTTPAPIServerStreamRRM3U8)
	//public.GET("/stream/:uuid/channel/:channel/hls/rr/:s/:e/:seq/file.ts", HTTPAPIServerStreamRRTS)
	//HLS LL
	public.GET("/stream/:uuid/channel/:channel/hlsll/live/index.m3u8", HTTPAPIServerStreamHLSLLM3U8)
	public.GET("/stream/:uuid/channel/:channel/hlsll/live/init.mp4", HTTPAPIServerStreamHLSLLInit)
	public.GET("/stream/:uuid/channel/:channel/hlsll/live/segment/:segment/:any", HTTPAPIServerStreamHLSLLM4Segment)
	public.GET("/stream/:uuid/channel/:channel/hlsll/live/fragment/:segment/:fragment/:any", HTTPAPIServerStreamHLSLLM4Fragment)
	//MSE
	public.GET("/stream/:uuid/channel/:channel/mse", HTTPAPIServerStreamMSE)

	// WebRTC
	public.POST("/stream/:uuid/channel/:channel/webrtc", HTTPAPIServerStreamWebRTC)
	//Save fragment to mp4
	public.GET("/stream/:uuid/channel/:channel/save/mp4/fragment/:duration", HTTPAPIServerStreamSaveToMP4)

	// FFMPEG 녹화
	// 녹화 관련 엔드포인트 추가
	privat.POST("/stream/:uuid/channel/:channel/recording/start", HTTPAPIServerRecordingStart)
	privat.POST("/stream/:uuid/channel/:channel/recording/stop", HTTPAPIServerRecordingStop)
	privat.GET("/stream/:uuid/channel/:channel/recording/play", HTTPAPIServerRecordingPlay)

	// 다중 스트림 녹화 API
	public.POST("/streams/recording/start", HTTPAPIServerRecordingMultiStart)
	public.POST("/streams/recording/stop", HTTPAPIServerRecordingMultiStop)

	// 녹화 파일 목록 조회
	public.GET("/stream/recording/list", HTTPAPIServerRecordingListByDate)

	// 암호화 키 제공 (public - hls.js가 자동 요청)
	// 따로 쿼리파라미터 추가를 못하므로 URL에 이렇게 입력하는 수 밖에 없음.
	public.GET("/stream/:uuid/channel/:channel/recording/key", HTTPAPIServerRecordingKey)

	// EventView
	public.GET("/stream/recordingStream", HTTPAPIServerRecordingStreaming)

	// m3u8 파일 직접 서빙
	public.GET("/stream/recording/m3u8", HTTPAPIServerRecordingM3U8File)

	// privat.GET("/stream/:uuid/channel/:channel/recording/list", HTTPAPIServerRecordingList)
	// privat.POST("/stream/:uuid/channel/:channel/recording/:recording_id/webrtc", HTTPAPIServerRecordingWebRTC)

	/*
		Monitoring API
	*/
	public.GET("/monitoring/dashboard", HTTPAPIMonitoringDashboard)
	public.GET("/api/monitoring/data", HTTPAPIMonitoringData)
	public.GET("/api/monitoring/system", HTTPAPIMonitoringSystem)
	public.GET("/api/monitoring/streams", HTTPAPIMonitoringStreams)
	public.GET("/api/monitoring/stream/:uuid", HTTPAPIMonitoringStream)
	public.GET("/api/monitoring/alerts", HTTPAPIMonitoringAlerts)
	public.GET("/api/monitoring/history", HTTPAPIMonitoringHistory)
	public.GET("/api/monitoring/stats", HTTPAPIMonitoringStats)

	/*
		HTTPS Mode Cert
		# Key considerations for algorithm "RSA" ≥ 2048-bit
		openssl genrsa -out server.key 2048

		# Key considerations for algorithm "ECDSA" ≥ secp384r1
		# List ECDSA the supported curves (openssl ecparam -list_curves)
		#openssl ecparam -genkey -name secp384r1 -out server.key
		#Generation of self-signed(x509) public key (PEM-encodings .pem|.crt) based on the private (.key)

		openssl req -new -x509 -sha256 -key server.key -out server.crt -days 3650
	*/
	var err error
	if Storage.ServerHTTPS() {
		// HTTPS 모드: HTTPS만 실행
		serverAddress := Storage.ServerICEServerIP()
		sslgen := initMediaSSLgen(serverAddress)
		// 수동 인증서 사용
		log.Printf("[INFO] [http_router] [HTTPAPIServer] [TLS] Starting HTTPS server with manual certificates")
		err = public.RunTLS(Storage.ServerHTTPPort(), sslgen.certpemfilepath, sslgen.keypemfilepath)

		if err != nil {
			log.Printf("[ERROR] [http_router] [HTTPAPIServer] [HTTPS] %s", err.Error())
			os.Exit(1)
		}
	} else {
		// HTTP 모드: HTTP만 실행
		log.Printf("[INFO] [http_router] [HTTPAPIServer] [HTTP] Starting HTTP server")
		err = public.Run(Storage.ServerHTTPPort())
		if err != nil {
			log.Printf("[ERROR] [http_router] [HTTPAPIServer] [HTTP] %s", err.Error())
			os.Exit(1)
		}
	}

}

// HTTPAPIServerIndex index file
func HTTPAPIServerIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "index",
	})

}

func HTTPAPIServerDocumentation(c *gin.Context) {
	c.HTML(http.StatusOK, "documentation.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "documentation",
	})
}

func HTTPAPIStreamList(c *gin.Context) {
	c.HTML(http.StatusOK, "stream_list.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "stream_list",
	})
}

func HTTPAPIPlayHls(c *gin.Context) {
	c.HTML(http.StatusOK, "play_hls.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "play_hls",
		"uuid":    c.Param("uuid"),
		"channel": c.Param("channel"),
	})
}
func HTTPAPIPlayMse(c *gin.Context) {
	c.HTML(http.StatusOK, "play_mse.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "play_mse",
		"uuid":    c.Param("uuid"),
		"channel": c.Param("channel"),
	})
}
func HTTPAPIPlayWebrtc(c *gin.Context) {
	c.HTML(http.StatusOK, "play_webrtc.tmpl", addICEConfig(gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "play_webrtc",
		"uuid":    c.Param("uuid"),
		"channel": c.Param("channel"),
	}))
}
func HTTPAPIAddStream(c *gin.Context) {
	c.HTML(http.StatusOK, "add_stream.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "add_stream",
	})
}
func HTTPAPIEditStream(c *gin.Context) {
	c.HTML(http.StatusOK, "edit_stream.tmpl", gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "edit_stream",
		"uuid":    c.Param("uuid"),
	})
}

func HTTPAPIMultiview(c *gin.Context) {
	c.HTML(http.StatusOK, "multiview.tmpl", addICEConfig(gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "multiview",
	}))
}

func HTTPAPIPlayAll(c *gin.Context) {
	c.HTML(http.StatusOK, "play_all.tmpl", addICEConfig(gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"page":    "play_all",
		"uuid":    c.Param("uuid"),
		"channel": c.Param("channel"),
	}))
}

// HLS 영상 서빙을 위한 FileServer 등록 (동적 변경)
func HTTPAPIServerRecordingStatic(c *gin.Context) {
	root := Storage.Server.Maintenance.RetentionRoot
	if root == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	requestPath := c.Param("filepath")
	if requestPath == "" || requestPath == "/" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	cleaned := filepath.Clean(requestPath)
	cleaned = strings.TrimPrefix(cleaned, "/")

	targetPath := filepath.Join(root, cleaned)

	absRoot, err := filepath.Abs(root)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !strings.HasPrefix(absTarget, absRoot) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	info, err := os.Stat(absTarget)
	if err != nil {
		if os.IsNotExist(err) {
			c.AbortWithStatus(http.StatusNotFound)
		} else {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	if info.IsDir() {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	c.File(absTarget)
}

// BaseRoot 등록 시 폴더 선택 편리하도록
func HTTPAPIMaintenanceDirectories(c *gin.Context) {
	requestedPath := c.Query("path")
	if requestedPath == "" {
		requestedPath = Storage.Server.Maintenance.BaseRoot
		if requestedPath == "" {
			requestedPath = "."
		}
	}

	absPath, err := filepath.Abs(requestedPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "지정한 경로가 폴더가 아닙니다"})
		return
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var directories []gin.H
	for _, entry := range entries {
		if entry.IsDir() {
			directories = append(directories, gin.H{
				"name": entry.Name(),
				"path": filepath.Join(absPath, entry.Name()),
			})
		}
	}

	sort.Slice(directories, func(i, j int) bool {
		return directories[i]["name"].(string) < directories[j]["name"].(string)
	})

	var parentPath string
	if parent := filepath.Dir(absPath); parent != absPath {
		parentPath = parent
	}

	c.JSON(http.StatusOK, gin.H{
		"currentPath": absPath,
		"parentPath":  parentPath,
		"directories": directories,
	})
}

type MultiViewOptions struct {
	Grid   int                             `json:"grid"`
	Player map[string]MultiViewOptionsGrid `json:"player"`
}
type MultiViewOptionsGrid struct {
	UUID       string `json:"uuid"`
	Channel    int    `json:"channel"`
	PlayerType string `json:"playerType"`
}

func HTTPAPIFullScreenMultiView(c *gin.Context) {
	var createParams MultiViewOptions
	err := c.ShouldBindJSON(&createParams)
	if err != nil {
		log.Printf("[ERROR] [http_page] [HTTPAPIFullScreenMultiView] [BindJSON] %s", err.Error())
	}
	log.Printf("[INFO] [http_page] [HTTPAPIFullScreenMultiView] [Options] %v", createParams)
	c.HTML(http.StatusOK, "fullscreenmulti.tmpl", addICEConfig(gin.H{
		"port":    Storage.ServerHTTPPort(),
		"streams": Storage.Streams,
		"version": time.Now().String(),
		"options": createParams,
		"page":    "fullscreenmulti",
		"query":   c.Request.URL.Query(),
	}))
}

// CrossOrigin Access-Control-Allow-Origin any methods
func CrossOrigin() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
