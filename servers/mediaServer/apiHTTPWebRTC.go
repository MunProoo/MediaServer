package main

import (
	"encoding/json"
	"html/template"
	"time"

	"log"

	webrtc "github.com/deepch/vdk/format/webrtcv3"
	"github.com/gin-gonic/gin"
)

// HTTPAPIServerStreamWebRTC stream video over WebRTC
func HTTPAPIServerStreamWebRTC(c *gin.Context) {
	if !Storage.StreamChannelExist(c.Param("uuid"), c.Param("channel")) {
		c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		log.Printf("[ERROR] [http_webrtc] [HTTPAPIServerStreamWebRTC] [StreamChannelExist] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), ErrorStreamNotFound.Error())
		return
	}

	if !RemoteAuthorization("WebRTC", c.Param("uuid"), c.Param("channel"), c.Query("token"), c.ClientIP()) {
		log.Printf("[ERROR] [http_webrtc] [HTTPAPIServerStreamWebRTC] [RemoteAuthorization] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), ErrorStreamUnauthorized.Error())
		return
	}

	Storage.StreamChannelRun(c.Param("uuid"), c.Param("channel"))
	codecs, err := Storage.StreamChannelCodecs(c.Param("uuid"), c.Param("channel"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_webrtc] [HTTPAPIServerStreamWebRTC] [StreamCodecs] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}

	// 코덱이 있지만 RTSP가 OFFLINE 상태일 수 있음 (이전 연결의 코덱이 남아있는 경우)
	// StreamChannelRun() 호출 후 RTSP 연결 시도가 완료될 시간을 대기하고 상태 확인
	channelInfo, err := Storage.StreamChannelInfo(c.Param("uuid"), c.Param("channel"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_webrtc] [HTTPAPIServerStreamWebRTC] [StreamChannelInfo] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}

	// RTSP가 OFFLINE 상태이면 연결 시도 중이거나 실패한 상태
	// DialTimeout이 3초이므로 최소 3초 + 여유시간 대기
	if channelInfo.Status == OFFLINE {
		time.Sleep(4 * time.Second)

		// 다시 상태 확인
		channelInfo, err = Storage.StreamChannelInfo(c.Param("uuid"), c.Param("channel"))
		if err != nil {
			c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
			log.Printf("[ERROR] [http_webrtc] [HTTPAPIServerStreamWebRTC] [StreamChannelInfo] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
			return
		}

		// 여전히 OFFLINE이면 RTSP 연결 실패
		if channelInfo.Status == OFFLINE {
			c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamChannelCodecNotFound.Error()})
			log.Printf("[ERROR] [http_webrtc] [HTTPAPIServerStreamWebRTC] [StreamOffline] stream=%s channel=%s: RTSP connection failed or stream is offline", c.Param("uuid"), c.Param("channel"))
			return
		}
	}

	muxerWebRTC := webrtc.NewMuxer(webrtc.Options{ICEServers: Storage.ServerICEServers(), ICEUsername: Storage.ServerICEUsername(), ICECredential: Storage.ServerICECredential(), PortMin: Storage.ServerWebRTCPortMin(), PortMax: Storage.ServerWebRTCPortMax()})
	answer, err := muxerWebRTC.WriteHeader(codecs, c.PostForm("data"))
	if err != nil {
		c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_webrtc] [HTTPAPIServerStreamWebRTC] [WriteHeader] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	_, err = c.Writer.Write([]byte(answer))
	if err != nil {
		c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_webrtc] [HTTPAPIServerStreamWebRTC] [Write] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	go func() {
		cid, ch, _, err := Storage.ClientAdd(c.Param("uuid"), c.Param("channel"), WEBRTC)
		if err != nil {
			c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
			log.Printf("[ERROR] [http_webrtc] [HTTPAPIServerStreamWebRTC] [ClientAdd] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
			return
		}
		defer Storage.ClientDelete(c.Param("uuid"), cid, c.Param("channel"))
		var videoStart bool
		noVideo := time.NewTimer(20 * time.Second)
		for {
			select {
			case <-noVideo.C:
				//				c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNoVideo.Error()})
				log.Printf("[ERROR] [http_webrtc] [HTTPAPIServerStreamWebRTC] [ErrorStreamNoVideo] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), ErrorStreamNoVideo.Error())
				return
			case pck := <-ch:
				if pck.IsKeyFrame {
					noVideo.Reset(10 * time.Second)
					videoStart = true
				}
				if !videoStart {
					continue
				}
				err = muxerWebRTC.WritePacket(*pck)
				if err != nil {
					log.Printf("[ERROR] [http_webrtc] [HTTPAPIServerStreamWebRTC] [WritePacket] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
					return
				}
			}
		}
	}()
}

func addICEConfig(data gin.H) gin.H {
	iceServersJSON := "[]"
	if servers := Storage.ServerICEServers(); len(servers) > 0 {
		if payload, err := json.Marshal(servers); err == nil {
			iceServersJSON = string(payload)
		}
	}
	data["iceServers"] = template.JS(iceServersJSON)
	data["iceUsername"] = Storage.ServerICEUsername()
	data["iceCredential"] = Storage.ServerICECredential()
	return data
}
