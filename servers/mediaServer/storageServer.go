package main

import (
	"path/filepath"

	"github.com/sirupsen/logrus"
)

var (
	//Default www static file dir
	DefaultHTTPDir = "web"
)

// ServerHTTPDir
func (obj *StorageST) ServerHTTPDir() string {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	if filepath.Clean(obj.Server.HTTPDir) == "." {
		return DefaultHTTPDir
	}
	return filepath.Clean(obj.Server.HTTPDir)
}

// EffectiveBaseRoot returns the absolute path currently used as base root.
func (obj *StorageST) EffectiveBaseRoot() string {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	baseRoot := obj.Server.Maintenance.BaseRoot
	if baseRoot == "" {
		baseRoot = "."
	}
	if abs, err := filepath.Abs(baseRoot); err == nil {
		return abs
	}
	return baseRoot
}

// ServerHTTPDebug read debug options
func (obj *StorageST) ServerHTTPDebug() bool {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	return obj.Server.HTTPDebug
}

// ServerLogLevel read debug options
func (obj *StorageST) ServerLogLevel() logrus.Level {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	return obj.Server.LogLevel
}

// ServerHTTPDemo read demo options
func (obj *StorageST) ServerHTTPDemo() bool {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	return obj.Server.HTTPDemo
}

// ServerHTTPLogin read Login options
func (obj *StorageST) ServerHTTPLogin() string {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	return obj.Server.HTTPLogin
}

// ServerHTTPPassword read Password options
func (obj *StorageST) ServerHTTPPassword() string {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	return obj.Server.HTTPPassword
}

// ServerHTTPPort read HTTP Port options
func (obj *StorageST) ServerHTTPPort() string {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	return obj.Server.HTTPPort
}

// ServerRTSPPort read HTTP Port options
func (obj *StorageST) ServerRTSPPort() string {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	return obj.Server.RTSPPort
}

// ServerHTTPS read HTTPS Port options
func (obj *StorageST) ServerHTTPS() bool {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	return obj.Server.HTTPS
}

// ServerHTTPSPort read HTTPS Port options
func (obj *StorageST) ServerHTTPSPort() string {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	return obj.Server.HTTPSPort
}

// ServerHTTPSAutoTLSEnable read HTTPS Port options
func (obj *StorageST) ServerHTTPSAutoTLSEnable() bool {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	return obj.Server.HTTPSAutoTLSEnable
}

// ServerHTTPSAutoTLSName read HTTPS Port options
func (obj *StorageST) ServerHTTPSAutoTLSName() string {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	return obj.Server.HTTPSAutoTLSName
}

// ServerHTTPSCert read HTTPS Cert options
func (obj *StorageST) ServerHTTPSCert() string {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	return obj.Server.HTTPSCert
}

// ServerHTTPSKey read HTTPS Key options
func (obj *StorageST) ServerHTTPSKey() string {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	return obj.Server.HTTPSKey
}

// ServerICEServers read ICE servers
func (obj *StorageST) ServerICEServers() []string {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	return obj.Server.ICEServers
}

// ServerICEServerIP ICE 서버 설정에서 첫 번째 IP 주소 추출
func (obj *StorageST) ServerICEServerIP() string {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	return extractFirstIPFromICEServers(obj.Server.ICEServers)
}

// ServerICEServers read ICE username
func (obj *StorageST) ServerICEUsername() string {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	return obj.Server.ICEUsername
}

// ServerICEServers read ICE credential
func (obj *StorageST) ServerICECredential() string {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	return obj.Server.ICECredential
}

// ServerTokenEnable read HTTPS Key options
func (obj *StorageST) ServerTokenEnable() bool {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	return obj.Server.Token.Enable
}

// ServerTokenBackend read HTTPS Key options
func (obj *StorageST) ServerTokenBackend() string {
	obj.mutex.RLock()
	defer obj.mutex.RUnlock()
	return obj.Server.Token.Backend
}

// ServerWebRTCPortMin read WebRTC Port Min
func (obj *StorageST) ServerWebRTCPortMin() uint16 {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	return obj.Server.WebRTCPortMin
}

// ServerWebRTCPortMax read WebRTC Port Max
func (obj *StorageST) ServerWebRTCPortMax() uint16 {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()
	return obj.Server.WebRTCPortMax
}

// Server Config Edit
func (obj *StorageST) ServerEdit(val ServerST) error {
	obj.mutex.Lock()
	defer obj.mutex.Unlock()

	if val.Debug {
		obj.Server.Debug = val.Debug
	}
	if val.HTTPDebug {
		obj.Server.HTTPDebug = val.HTTPDebug
	}
	if val.HTTPDemo {
		obj.Server.HTTPDemo = val.HTTPDemo
	}
	if len(val.HTTPLogin) > 0 {
		obj.Server.HTTPLogin = val.HTTPLogin
	}
	if len(val.HTTPPassword) > 0 {
		obj.Server.HTTPPassword = val.HTTPPassword
	}
	if len(val.HTTPPort) > 0 {
		obj.Server.HTTPPort = val.HTTPPort
	}

	// HTTPS
	if val.HTTPS {
		obj.Server.HTTPS = val.HTTPS
	}
	if len(val.HTTPSPort) > 0 {
		obj.Server.HTTPSPort = val.HTTPSPort
	}

	// ICE
	if len(val.ICEServers) > 0 {
		obj.Server.ICEServers = val.ICEServers
	}
	if len(val.ICEUsername) > 0 {
		obj.Server.ICEUsername = val.ICEUsername
	}
	if len(val.ICECredential) > 0 {
		obj.Server.ICECredential = val.ICECredential
	}

	// RTSP
	if len(val.RTSPPort) > 0 {
		obj.Server.RTSPPort = val.RTSPPort
	}

	// WebRTC
	if val.WebRTCPortMin != 0 {
		obj.Server.WebRTCPortMin = val.WebRTCPortMin
	}
	if val.WebRTCPortMax != 0 {
		obj.Server.WebRTCPortMax = val.WebRTCPortMax
	}

	err := obj.SaveConfig()
	if err != nil {
		return err
	}
	return nil
}
