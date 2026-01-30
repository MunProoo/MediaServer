package main

import (
	"log"

	"github.com/gin-gonic/gin"
)

// HTTPAPIServerStreamChannelCodec function return codec info struct
func HTTPAPIServerStreamChannelCodec(c *gin.Context) {
	if !Storage.StreamChannelExist(c.Param("uuid"), c.Param("channel")) {
		c.IndentedJSON(500, Message{Status: 0, Payload: ErrorStreamNotFound.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamChannelCodec] [StreamChannelExist] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), ErrorStreamNotFound.Error())
		return
	}
	codecs, err := Storage.StreamChannelCodecs(c.Param("uuid"), c.Param("channel"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamChannelCodec] [StreamChannelCodec] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	c.IndentedJSON(200, Message{Status: 1, Payload: codecs})
}

// HTTPAPIServerStreamChannelInfo function return stream info struct
func HTTPAPIServerStreamChannelInfo(c *gin.Context) {
	info, err := Storage.StreamChannelInfo(c.Param("uuid"), c.Param("channel"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamChannelInfo] [StreamChannelInfo] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	c.IndentedJSON(200, Message{Status: 1, Payload: info})
}

// HTTPAPIServerStreamChannelReload function reload stream
func HTTPAPIServerStreamChannelReload(c *gin.Context) {
	err := Storage.StreamChannelReload(c.Param("uuid"), c.Param("channel"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamChannelReload] [StreamChannelReload] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	c.IndentedJSON(200, Message{Status: 1, Payload: Success})
}

// HTTPAPIServerStreamChannelEdit function edit stream
func HTTPAPIServerStreamChannelEdit(c *gin.Context) {
	var payload ChannelST
	err := c.BindJSON(&payload)
	if err != nil {
		c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamChannelEdit] [BindJSON] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	err = Storage.StreamChannelEdit(c.Param("uuid"), c.Param("channel"), payload)
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamChannelEdit] [StreamChannelEdit] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}

	// Stream 정보 바뀌면 녹화 재시작
	Storage.RestartRecordingStream(c.Param("uuid"), c.Param("channel"))

	c.IndentedJSON(200, Message{Status: 1, Payload: Success})
}

// HTTPAPIServerStreamChannelDelete function delete stream
func HTTPAPIServerStreamChannelDelete(c *gin.Context) {
	err := Storage.StreamChannelDelete(c.Param("uuid"), c.Param("channel"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamChannelDelete] [StreamChannelDelete] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	c.IndentedJSON(200, Message{Status: 1, Payload: Success})
}

// HTTPAPIServerStreamChannelAdd function add new stream
func HTTPAPIServerStreamChannelAdd(c *gin.Context) {
	var payload ChannelST
	err := c.BindJSON(&payload)
	if err != nil {
		c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamChannelAdd] [BindJSON] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	err = Storage.StreamChannelAdd(c.Param("uuid"), c.Param("channel"), payload)
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamChannelAdd] [StreamChannelAdd] stream=%s channel=%s: %s", c.Param("uuid"), c.Param("channel"), err.Error())
		return
	}
	c.IndentedJSON(200, Message{Status: 1, Payload: Success})
}
