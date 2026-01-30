package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-version"

	"github.com/imdario/mergo"

	"github.com/liip/sheriff"
)

var configFile string

// NewStreamCore do load config file
func NewStreamCore() *StorageST {
	// 서비스로 등록 시에 시작 위치는 C:\\windows\\System32 임. 그래서 그냥 config.json하면 못읽고 exe 위치에서 읽도록 해야함!!!
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	configFile = filepath.Join(exeDir, "msConfig.cnf")

	var tmp StorageST

	data, err := os.ReadFile(configFile)
	// 없으면 만들어내는 코드. 지울까..
	if err != nil {
		// config 파일이 없으면 기본값으로 생성
		if os.IsNotExist(err) {
			log.Printf("make default config....")
			// out.Printe(out.LogArg{"pn": "main", "func": "NewStreamCore", "text": "make default config...", "file": configFile})

			tmp = createDefaultConfig()
			if err := saveConfigToFile(configFile, &tmp); err != nil {
				log.Printf("failed to config. %s", err.Error())
				log.Printf("[ERROR] [main] [NewStreamCore] %s", err.Error())
				os.Exit(1)
			}
		} else {
			log.Printf("cannot open config. %s", err.Error())
			log.Printf("[ERROR] [main] [NewStreamCore] cannot open config file: err=%s", err.Error())
			os.Exit(1)
		}
	} else {
		// config 파일이 있으면 읽어서 파싱
		err = json.Unmarshal(data, &tmp)
		if err != nil {
			log.Printf("config unmarshal failed. %s", err.Error())
			log.Printf("[ERROR] [main] [NewStreamCore] %s", err.Error())
			os.Exit(1)
		}
	}

	// 기존 코드
	for i, i2 := range tmp.Streams {
		for i3, i4 := range i2.Channels {
			channel := tmp.ChannelDefaults
			err = mergo.Merge(&channel, i4)
			if err != nil {
				log.Printf("[ERROR] [main] [NewStreamCore] [Merge] %s", err.Error())
				os.Exit(1)
			}
			channel.clients = make(map[string]ClientST)
			channel.ack = time.Now().Add(-255 * time.Hour)
			channel.hlsSegmentBuffer = make(map[int]SegmentOld)
			channel.signals = make(chan int, 100)
			i2.Channels[i3] = channel
		}
		tmp.Streams[i] = i2
	}

	// 녹화 추가
	tmp.Recordings = make(map[string]*RecordingST)
	tmp.setRetentionRoot()
	return &tmp
}

// Client Delete Client
func (obj *StorageST) SaveConfig() error {
	v2, err := version.NewVersion("2.0.0")
	if err != nil {
		return err
	}
	data, err := sheriff.Marshal(&sheriff.Options{
		Groups:     []string{"config"},
		ApiVersion: v2,
	}, obj)
	if err != nil {
		return err
	}
	res, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	err = os.WriteFile(configFile, res, 0644)
	if err != nil {
		log.Printf("[ERROR] [main] [SaveConfig] %s", err.Error())
		return err
	}
	log.Printf("[INFO] [main] [SaveConfig] successfully saved.")
	return nil
}

// createDefaultConfig 기본 설정 생성
func createDefaultConfig() StorageST {
	return StorageST{
		Server: ServerST{
			Debug: true,
			// LogLevel:           logrus.InfoLevel,
			HTTPDemo:           true,
			HTTPDebug:          false,
			HTTPLogin:          "admin",
			HTTPPassword:       "admin",
			HTTPDir:            "media_web",
			HTTPPort:           ":8083",
			RTSPPort:           ":5541",
			HTTPS:              false,
			HTTPSPort:          ":443",
			HTTPSCert:          "server.crt",
			HTTPSKey:           "server.key",
			HTTPSAutoTLSEnable: false,
			HTTPSAutoTLSName:   "",
			ICEServers:         []string{},
			ICEUsername:        "",
			ICECredential:      "",
			Token: Token{
				Enable:  false,
				Backend: "",
			},
			WebRTCPortMin: 49152,
			WebRTCPortMax: 49220,
			FFMPEGPath:    "./external_tools/",
			Maintenance: MaintenanceConfig{
				DiskCheckInterval:      1, // 1시간마다 디스크 체크
				RetentionDays:          30,
				RetentionCapacity:      500,
				BaseRoot:               ".",
				DefaultSafetyFreeSpace: 5, // 디스크에 여유용량이 5GB가 안된다면 정리하도록
			},
		},
		Streams:         make(map[string]StreamST),
		ChannelDefaults: ChannelST{},
		Recordings:      make(map[string]*RecordingST),
	}
}

// saveConfigToFile config를 파일로 저장
func saveConfigToFile(filePath string, config *StorageST) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// 녹화 경로 재설정 시 재설정 필요 @@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@
func (obj *StorageST) setRetentionRoot() {
	cfg := obj.Server.Maintenance
	baseRoot := cfg.BaseRoot
	if baseRoot == "" {
		baseRoot = "."
	}

	if err := os.MkdirAll(baseRoot, 0755); err != nil {
		log.Printf("[ERROR] [maintenance] [checkRetentionPolicies] failed to create retention base directory: path=%s err=%v", baseRoot, err)
		return
	}

	subDir := strings.TrimLeft(RecordingDir, "/\\")
	root := filepath.Join(baseRoot, subDir)
	if err := os.MkdirAll(root, 0755); err != nil {
		log.Printf("[ERROR] [maintenance] [checkRetentionPolicies] failed to create retention recording directory: path=%s err=%v", root, err)
		return
	}

	obj.Server.Maintenance.RetentionRoot = root
}
