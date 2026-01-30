package main

import (
	"log"

	"github.com/deepch/vdk/format/mp4f"

	"github.com/gin-gonic/gin"
)

// HTTPAPIServerStreamHLSLLInit send client ts segment
func HTTPAPIServerStreamHLSLLInit(c *gin.Context) {
	if !Storage.StreamChannelExist(c.Param("uuid"), c.Param("channel")) {
		c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLInit] [StreamChannelExist] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), ErrorStreamNotFound.Error())
		return
	}

	if !RemoteAuthorization("HLS", c.Param("uuid"), c.Param("channel"), c.Query("token"), c.ClientIP()) {
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLInit] [RemoteAuthorization] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), ErrorStreamUnauthorized.Error())
		return
	}

	c.Header("Content-Type", "application/x-mpegURL")
	Storage.StreamChannelRun(c.Param("uuid"), c.Param("channel"))
	codecs, err := Storage.StreamChannelCodecs(c.Param("uuid"), c.Param("channel"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLInit] [StreamChannelCodecs] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	Muxer := mp4f.NewMuxer(nil)
	err = Muxer.WriteHeader(codecs)
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLInit] [WriteHeader] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	c.Header("Content-Type", "video/mp4")
	_, buf := Muxer.GetInit(codecs)
	_, err = c.Writer.Write(buf)
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLInit] [Write] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
}

// HTTPAPIServerStreamHLSLLM3U8 send client m3u8 play list
func HTTPAPIServerStreamHLSLLM3U8(c *gin.Context) {
	if !Storage.StreamChannelExist(c.Param("uuid"), c.Param("channel")) {
		c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLM3U8] [StreamChannelExist] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), ErrorStreamNotFound.Error())
		return
	}
	c.Header("Content-Type", "application/x-mpegURL")
	Storage.StreamChannelRun(c.Param("uuid"), c.Param("channel"))
	index, err := Storage.HLSMuxerM3U8(c.Param("uuid"), c.Param("channel"), stringToInt(c.DefaultQuery("_HLS_msn", "-1")), stringToInt(c.DefaultQuery("_HLS_part", "-1")))
	if err != nil {
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLM3U8] [HLSMuxerM3U8] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), ErrorStreamNotFound.Error())
		return
	}
	_, err = c.Writer.Write([]byte(index))
	if err != nil {
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLM3U8] [Write] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), ErrorStreamNotFound.Error())
		return
	}
}

// HTTPAPIServerStreamHLSLLM4Segment send client ts segment
func HTTPAPIServerStreamHLSLLM4Segment(c *gin.Context) {
	c.Header("Content-Type", "video/mp4")
	if !Storage.StreamChannelExist(c.Param("uuid"), c.Param("channel")) {
		c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLM4Segment] [StreamChannelExist] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), ErrorStreamNotFound.Error())
		return
	}
	codecs, err := Storage.StreamChannelCodecs(c.Param("uuid"), c.Param("channel"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLM4Segment] [StreamChannelCodecs] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	if codecs == nil {
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLM4Segment] [StreamCodecs] Codec Null: stream=%s channel=%s", c.Param("uuid"), c.Param("channel"))
		return
	}
	Muxer := mp4f.NewMuxer(nil)
	err = Muxer.WriteHeader(codecs)
	if err != nil {
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLM4Segment] [WriteHeader] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	seqData, err := Storage.HLSMuxerSegment(c.Param("uuid"), c.Param("channel"), stringToInt(c.Param("segment")))
	if err != nil {
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLM4Segment] [HLSMuxerSegment] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	for _, v := range seqData {
		err = Muxer.WritePacket4(*v)
		if err != nil {
			log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLM4Segment] [WritePacket4] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
			return
		}
	}
	buf := Muxer.Finalize()
	_, err = c.Writer.Write(buf)
	if err != nil {
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLM4Segment] [Write] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
}

// HTTPAPIServerStreamHLSLLM4Fragment send client ts segment
func HTTPAPIServerStreamHLSLLM4Fragment(c *gin.Context) {
	c.Header("Content-Type", "video/mp4")
	if !Storage.StreamChannelExist(c.Param("uuid"), c.Param("channel")) {
		c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLM4Fragment] [StreamChannelExist] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), ErrorStreamNotFound.Error())
		return
	}
	codecs, err := Storage.StreamChannelCodecs(c.Param("uuid"), c.Param("channel"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLM4Fragment] [StreamChannelCodecs] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	if codecs == nil {
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLM4Fragment] [StreamCodecs] Codec Null: stream=%s channel=%s", c.Param("uuid"), c.Param("channel"))
		return
	}
	Muxer := mp4f.NewMuxer(nil)
	err = Muxer.WriteHeader(codecs)
	if err != nil {
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLM4Fragment] [WriteHeader] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	seqData, err := Storage.HLSMuxerFragment(c.Param("uuid"), c.Param("channel"), stringToInt(c.Param("segment")), stringToInt(c.Param("fragment")))
	if err != nil {
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLM4Fragment] [HLSMuxerFragment] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	for _, v := range seqData {
		err = Muxer.WritePacket4(*v)
		if err != nil {
			log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLM4Fragment] [WritePacket4] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
			return
		}
	}
	buf := Muxer.Finalize()
	_, err = c.Writer.Write(buf)
	if err != nil {
		log.Printf("[ERROR] [http_hlsll] [HTTPAPIServerStreamHLSLLM4Fragment] [Write] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
}
