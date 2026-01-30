package main

import (
	"time"

	"github.com/gobwas/ws/wsutil"

	"github.com/gobwas/ws"

	"github.com/gin-gonic/gin"

	"log"

	"github.com/deepch/vdk/format/mp4f"
)

// HTTPAPIServerStreamMSE func
func HTTPAPIServerStreamMSE(c *gin.Context) {
	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		return
	}

	defer func() {
		err = conn.Close()
		if err != nil {
			log.Printf("[ERROR] [http_mse] [HTTPAPIServerStreamMSE] [Close] stream=%s channel=%s: %v", c.Param("uuid"), c.Param("channel"), err)
		}
		log.Println("Client Full Exit")
	}()
	if !Storage.StreamChannelExist(c.Param("uuid"), c.Param("channel")) {
		log.Printf("[ERROR] [http_mse] [HTTPAPIServerStreamMSE] [StreamChannelExist] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), ErrorStreamNotFound.Error())
		return
	}

	if !RemoteAuthorization("WS", c.Param("uuid"), c.Param("channel"), c.Query("token"), c.ClientIP()) {
		log.Printf("[ERROR] [http_mse] [HTTPAPIServerStreamMSE] [RemoteAuthorization] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), ErrorStreamUnauthorized.Error())
		return
	}

	Storage.StreamChannelRun(c.Param("uuid"), c.Param("channel"))
	err = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		log.Printf("[ERROR] [http_mse] [HTTPAPIServerStreamMSE] [SetWriteDeadline] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	cid, ch, _, err := Storage.ClientAdd(c.Param("uuid"), c.Param("channel"), MSE)
	if err != nil {
		log.Printf("[ERROR] [http_mse] [HTTPAPIServerStreamMSE] [ClientAdd] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	defer Storage.ClientDelete(c.Param("uuid"), cid, c.Param("channel"))
	codecs, err := Storage.StreamChannelCodecs(c.Param("uuid"), c.Param("channel"))
	if err != nil {
		log.Printf("[ERROR] [http_mse] [HTTPAPIServerStreamMSE] [StreamCodecs] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	muxerMSE := mp4f.NewMuxer(nil)
	err = muxerMSE.WriteHeader(codecs)
	if err != nil {
		log.Printf("[ERROR] [http_mse] [HTTPAPIServerStreamMSE] [WriteHeader] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	meta, init := muxerMSE.GetInit(codecs)
	err = wsutil.WriteServerMessage(conn, ws.OpBinary, append([]byte{9}, meta...))
	if err != nil {
		log.Printf("[ERROR] [http_mse] [HTTPAPIServerStreamMSE] [Send] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	err = wsutil.WriteServerMessage(conn, ws.OpBinary, init)
	if err != nil {
		log.Printf("[ERROR] [http_mse] [HTTPAPIServerStreamMSE] [Send] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	var videoStart bool
	controlExit := make(chan bool, 10)
	noClient := time.NewTimer(10 * time.Second)
	go func() {
		defer func() {
			controlExit <- true
		}()
		for {
			header, _, err := wsutil.NextReader(conn, ws.StateServerSide)
			if err != nil {
				log.Printf("[ERROR] [http_mse] [HTTPAPIServerStreamMSE] [Receive] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
				return
			}
			switch header.OpCode {
			case ws.OpPong:
				noClient.Reset(10 * time.Second)
			case ws.OpClose:
				return
			}
		}
	}()
	noVideo := time.NewTimer(10 * time.Second)
	pingTicker := time.NewTicker(500 * time.Millisecond)
	defer pingTicker.Stop()
	defer log.Println("client exit")
	for {
		select {

		case <-pingTicker.C:
			err = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
			if err != nil {
				return
			}
			buf, err := ws.CompileFrame(ws.NewPingFrame(nil))
			if err != nil {
				return
			}
			_, err = conn.Write(buf)
			if err != nil {
				return
			}
		case <-controlExit:
			log.Printf("[INFO] [http_mse] [HTTPAPIServerStreamMSE] [controlExit] Client Reader Exit: stream=%s channel=%s", c.Param("uuid"), c.Param("channel"))
			return
		case <-noClient.C:
			log.Printf("[INFO] [http_mse] [HTTPAPIServerStreamMSE] [ErrorClientOffline] Client OffLine Exit: stream=%s channel=%s", c.Param("uuid"), c.Param("channel"))
			return
		case <-noVideo.C:
			log.Printf("[ERROR] [http_mse] [HTTPAPIServerStreamMSE] [ErrorStreamNoVideo] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), ErrorStreamNoVideo.Error())
			return
		case pck := <-ch:
			if pck.IsKeyFrame {
				noVideo.Reset(10 * time.Second)
				videoStart = true
			}
			if !videoStart {
				continue
			}
			ready, buf, err := muxerMSE.WritePacket(*pck, false)
			if err != nil {
				log.Printf("[ERROR] [http_mse] [HTTPAPIServerStreamMSE] [WritePacket] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
				return
			}
			if ready {
				err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err != nil {
					log.Printf("[ERROR] [http_mse] [HTTPAPIServerStreamMSE] [SetWriteDeadline] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
					return
				}
				//err = websocket.Message.Send(ws, buf)
				err = wsutil.WriteServerMessage(conn, ws.OpBinary, buf)
				if err != nil {
					log.Printf("[ERROR] [http_mse] [HTTPAPIServerStreamMSE] [Send] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
					return
				}
			}
		}
	}
}
