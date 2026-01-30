package main

import (
	"errors"
	"io"
	"net"
	"os/exec"
	"sync"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/sirupsen/logrus"
)

var Storage *StorageST

// Default stream  type
const (
	MSE = iota
	WEBRTC
	RTSP
)

// Default stream status type
const (
	OFFLINE = iota
	ONLINE
)

// Default Recording
const (
	RecordingDir = "ubio_recordings"
)

// Default stream errors
var (
	Success                         = "success"
	ErrorStreamNotFound             = errors.New("stream not found")
	ErrorStreamAlreadyExists        = errors.New("stream already exists")
	ErrorStreamChannelAlreadyExists = errors.New("stream channel already exists")
	ErrorStreamNotHLSSegments       = errors.New("stream hls not ts seq found")
	ErrorStreamNoVideo              = errors.New("stream no video")
	ErrorStreamNoClients            = errors.New("stream no clients")
	ErrorStreamRestart              = errors.New("stream restart")
	ErrorStreamStopCoreSignal       = errors.New("stream stop core signal")
	ErrorStreamStopRTSPSignal       = errors.New("stream stop rtsp signal")
	ErrorStreamChannelNotFound      = errors.New("stream channel not found")
	ErrorStreamChannelCodecNotFound = errors.New("stream channel codec not ready, possible stream offline")
	ErrorStreamsLen0                = errors.New("streams len zero")
	ErrorStreamUnauthorized         = errors.New("stream request unauthorized")
)

// StorageST main storage struct
type StorageST struct {
	mutex           sync.RWMutex
	Server          ServerST                `json:"server" groups:"api,config"`
	Streams         map[string]StreamST     `json:"streams,omitempty" groups:"api,config"`
	ChannelDefaults ChannelST               `json:"channel_defaults,omitempty" groups:"api,config"`
	Recordings      map[string]*RecordingST `json:"recordings"`
}

// ServerST server storage section
type ServerST struct {
	Debug              bool              `json:"debug" groups:"api,config"`
	LogLevel           logrus.Level      `json:"log_level" groups:"api,config"`
	HTTPDemo           bool              `json:"http_demo" groups:"api,config"`
	HTTPDebug          bool              `json:"http_debug" groups:"api,config"`
	HTTPLogin          string            `json:"http_login" groups:"api,config"`
	HTTPPassword       string            `json:"http_password" groups:"api,config"`
	HTTPDir            string            `json:"http_dir" groups:"api,config"`
	HTTPPort           string            `json:"http_port" groups:"api,config"`
	RTSPPort           string            `json:"rtsp_port" groups:"api,config"`
	HTTPS              bool              `json:"https" groups:"api,config"`
	HTTPSPort          string            `json:"https_port" groups:"api,config"`
	HTTPSCert          string            `json:"https_cert" groups:"api,config"`
	HTTPSKey           string            `json:"https_key" groups:"api,config"`
	HTTPSAutoTLSEnable bool              `json:"https_auto_tls" groups:"api,config"`
	HTTPSAutoTLSName   string            `json:"https_auto_tls_name" groups:"api,config"`
	ICEServers         []string          `json:"ice_servers" groups:"api,config"`
	ICEUsername        string            `json:"ice_username" groups:"api,config"`
	ICECredential      string            `json:"ice_credential" groups:"api,config"`
	Token              Token             `json:"token,omitempty" groups:"api,config"`
	WebRTCPortMin      uint16            `json:"webrtc_port_min" groups:"api,config"`
	WebRTCPortMax      uint16            `json:"webrtc_port_max" groups:"api,config"`
	FFMPEGPath         string            `json:"ffmpeg_path" groups:"api,config"`
	Maintenance        MaintenanceConfig `json:"maintenance" groups:"api,config"`
}

// Token auth
type Token struct {
	Enable  bool   `json:"enable" groups:"api,config"`
	Backend string `json:"backend" groups:"api,config"`
}

// ServerST stream storage section
type StreamST struct {
	Name     string               `json:"name,omitempty" groups:"api,config"`
	Channels map[string]ChannelST `json:"channels,omitempty" groups:"api,config"`
}

type ChannelST struct {
	Name               string `json:"name,omitempty" groups:"api,config"`
	URL                string `json:"url,omitempty" groups:"api,config"`
	OnDemand           bool   `json:"on_demand,omitempty" groups:"api,config"`
	Debug              bool   `json:"debug,omitempty" groups:"api,config"`
	Status             int    `json:"status,omitempty" groups:"api"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty" groups:"api,config"`
	Audio              bool   `json:"audio,omitempty" groups:"api,config"`
	OnRecording        bool   `json:"on_recording,omitempty" groups:"api,config"` // 현재 녹화 상태
	runLock            bool
	codecs             []av.CodecData
	sdp                []byte
	signals            chan int
	hlsSegmentBuffer   map[int]SegmentOld
	hlsSegmentNumber   int
	hlsSequence        int
	hlsLastDur         int
	clients            map[string]ClientST
	ack                time.Time
	hlsMuxer           *MuxerHLS `json:"-"`

	Recording *RecordingST `json:"recording,omitempty"` // Recording 제어를 위해 필요한 값 (ffmpeg 등..)
}

// ClientST client storage section
type ClientST struct {
	mode              int
	signals           chan int
	outgoingAVPacket  chan *av.Packet
	outgoingRTPPacket chan *[]byte
	socket            net.Conn
}

// SegmentOld HLS cache section
type SegmentOld struct {
	dur  time.Duration
	data []*av.Packet
}

// 녹화용 새로운 구조체 추가
type RecordingST struct {
	StreamID   string
	ChannelID  string
	StartTime  time.Time
	EndTime    time.Time
	Status     int // 0: 녹화중, 1: 완료, 2: 에러
	FFmpegCmd  *exec.Cmd
	StopSignal chan bool
	doneChan   chan bool      // 안전하게 종료됐다 전달
	pw         *io.PipeWriter // 종료 명령 전달 파이프

	// 종료 원인 추적
	StoppedByUser bool // true: 사용자가 의도적으로 중지, false: 오류로 인한 중지

	// HLS 관련 필드 추가
	SessionID     string // 녹화 세션 ID (시작 시간 기반)
	PlaylistPath  string // m3u8 플레이리스트 경로
	SegmentDir    string // 세그먼트 저장 디렉토리
	SegmentPrefix string // 세그먼트 파일명 접두사
	StreamName    string
}

// const (
// 	RecordingOn = iota
// 	RecordingOff
// 	RecordingErr
// )

type FFProbeOutput struct {
	Format struct {
		Duration string `json:"duration"`
		Tags     struct {
			CreationTime string `json:"creation_time"`
		} `json:"tags"`
	} `json:"format"`
}

// RecordingFileInfo 녹화 파일 정보 구조체
type RecordingFileInfo struct {
	SessionID string `json:"sessionId"`
	FileName  string `json:"fileName"`
	FilePath  string `json:"filePath"`
	StartTime string `json:"startTime"`
	Size      int64  `json:"size"`
	ChannelID string `json:"channelId"`
}

// MaintenanceConfig 유지보수 설정
type MaintenanceConfig struct {
	DiskCheckInterval time.Duration `json:"disk_check_interval" groups:"api,config"` // 디스크 체크 간격 (기본: 1시간)

	RetentionDays          int     `json:"retention_days" groups:"api,config"`            // 보관 기간(일)
	RetentionCapacity      float64 `json:"retention_capacity" groups:"api,config"`        // 보관 용량 (GB)
	BaseRoot               string  `json:"base_root" groups:"api,config"`                 // 기본 보관 저장소 (절대/상대 경로)
	DefaultSafetyFreeSpace float64 `json:"default_safety_free_space" groups:"api,config"` // 디스크 최소 여유 공간

	RetentionRoot string `json:"retention_root"` // 실제 녹화 경로
}

type MaintenanceSettingsForm struct {
	RetentionDays          int     `form:"retentionDays" binding:"required,gte=0"`
	RetentionCapacity      float64 `form:"retentionCapacity" binding:"required,gte=0"`
	BaseRoot               string  `form:"baseRoot" `
	DefaultSafetyFreeSpace float64 `form:"defaultSafetyFreeSpace" binding:"required,gte=0"`
}

const (
	RecordingOff = iota
	RecordingOn
	RecordingErr
)
