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
		noVideo := time.NewTimer(10 * time.Second)
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
