package main

import (
	"fmt"
	"os"
	"time"

	"log"

	"github.com/deepch/vdk/format/mp4"
	"github.com/gin-gonic/gin"
)

// HTTPAPIServerStreamSaveToMP4 func
func HTTPAPIServerStreamSaveToMP4(c *gin.Context) {
	var err error

	defer func() {
		if err != nil {
			log.Printf("[ERROR] [http_save_mp4] [HTTPAPIServerStreamSaveToMP4] [Close] stream=%s channel=%s: %v", c.Param("uuid"), c.Param("channel"), err)
		}
	}()

	if !Storage.StreamChannelExist(c.Param("uuid"), c.Param("channel")) {
		log.Printf("[ERROR] [http_save_mp4] [HTTPAPIServerStreamSaveToMP4] [StreamChannelExist] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), ErrorStreamNotFound.Error())
		return
	}

	if !RemoteAuthorization("save", c.Param("uuid"), c.Param("channel"), c.Query("token"), c.ClientIP()) {
		log.Printf("[ERROR] [http_save_mp4] [HTTPAPIServerStreamSaveToMP4] [RemoteAuthorization] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), ErrorStreamUnauthorized.Error())
		return
	}
	c.Writer.Write([]byte("await save started"))
	go func() {
		Storage.StreamChannelRun(c.Param("uuid"), c.Param("channel"))
		cid, ch, _, err := Storage.ClientAdd(c.Param("uuid"), c.Param("channel"), MSE)
		if err != nil {
			log.Printf("[ERROR] [http_save_mp4] [HTTPAPIServerStreamSaveToMP4] [ClientAdd] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
			return
		}

		defer Storage.ClientDelete(c.Param("uuid"), cid, c.Param("channel"))
		codecs, err := Storage.StreamChannelCodecs(c.Param("uuid"), c.Param("channel"))
		if err != nil {
			log.Printf("[ERROR] [http_save_mp4] [HTTPAPIServerStreamSaveToMP4] [StreamCodecs] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
			return
		}
		err = os.MkdirAll(fmt.Sprintf("save/%s/%s/", c.Param("uuid"), c.Param("channel")), 0755)
		if err != nil {
			log.Printf("[ERROR] [http_save_mp4] [HTTPAPIServerStreamSaveToMP4] [MkdirAll] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		}
		f, err := os.Create(fmt.Sprintf("save/%s/%s/%s.mp4", c.Param("uuid"), c.Param("channel"), time.Now().Format("20060102_150405")))
		if err != nil {
			log.Printf("[ERROR] [http_save_mp4] [HTTPAPIServerStreamSaveToMP4] [Create] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		}
		defer f.Close()

		muxer := mp4.NewMuxer(f)
		err = muxer.WriteHeader(codecs)
		if err != nil {
			log.Printf("[ERROR] [http_save_mp4] [HTTPAPIServerStreamSaveToMP4] [WriteHeader] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
			return
		}
		defer muxer.WriteTrailer()

		var videoStart bool
		controlExit := make(chan bool, 10)
		dur, err := time.ParseDuration(c.Param("duration"))
		if err != nil {
			log.Printf("[ERROR] [http_save_mp4] [HTTPAPIServerStreamSaveToMP4] [ParseDuration] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		}
		saveLimit := time.NewTimer(dur)
		noVideo := time.NewTimer(10 * time.Second)
		defer log.Println("client exit")
		for {
			select {
			case <-controlExit:
				log.Printf("[INFO] [http_save_mp4] [HTTPAPIServerStreamSaveToMP4] [controlExit] Client Reader Exit: stream=%s channel=%s", c.Param("uuid"), c.Param("channel"))
				return
			case <-saveLimit.C:
				log.Printf("[INFO] [http_save_mp4] [HTTPAPIServerStreamSaveToMP4] [saveLimit] Saved Limit End: stream=%s channel=%s", c.Param("uuid"), c.Param("channel"))
				return
			case <-noVideo.C:
				log.Printf("[ERROR] [http_save_mp4] [HTTPAPIServerStreamSaveToMP4] [ErrorStreamNoVideo] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), ErrorStreamNoVideo.Error())
				return
			case pck := <-ch:
				if pck.IsKeyFrame {
					noVideo.Reset(10 * time.Second)
					videoStart = true
				}
				if !videoStart {
					continue
				}
				if err = muxer.WritePacket(*pck); err != nil {
					return
				}
			}
		}
	}()
}
