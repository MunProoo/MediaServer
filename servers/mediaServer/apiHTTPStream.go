package main

import (
	"log"

	"github.com/gin-gonic/gin"
)

// HTTPAPIServerStreams function return stream list
func HTTPAPIServerStreams(c *gin.Context) {
	list, err := Storage.MarshalledStreamsList()
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		return
	}
	c.IndentedJSON(200, Message{Status: 1, Payload: list})
}

// HTTPAPIServerStreamsMultiControlAdd function add new stream's
func HTTPAPIServerStreamsMultiControlAdd(c *gin.Context) {
	var payload StorageST
	err := c.BindJSON(&payload)
	if err != nil {
		c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamsMultiControlAdd] [BindJSON] %s", err.Error())
		return
	}
	if payload.Streams == nil || len(payload.Streams) < 1 {
		c.IndentedJSON(400, Message{Status: 0, Payload: ErrorStreamsLen0.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamsMultiControlAdd] [len(payload)] %s", ErrorStreamsLen0.Error())
		return
	}
	var resp = make(map[string]Message)
	var FoundError bool
	for k, v := range payload.Streams {
		err = Storage.StreamAdd(k, v)
		if err != nil {
			log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamsMultiControlAdd] [StreamAdd] stream=%s: %s", k, err.Error())
			resp[k] = Message{Status: 0, Payload: err.Error()}
			FoundError = true
		} else {
			resp[k] = Message{Status: 1, Payload: Success}
		}
	}
	if FoundError {
		c.IndentedJSON(200, Message{Status: 0, Payload: resp})
	} else {
		c.IndentedJSON(200, Message{Status: 1, Payload: resp})
	}
}

// HTTPAPIServerStreamsMultiControlDelete function delete stream's
func HTTPAPIServerStreamsMultiControlDelete(c *gin.Context) {
	var payload []string
	err := c.BindJSON(&payload)
	if err != nil {
		c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamsMultiControlDelete] [BindJSON] %s", err.Error())
		return
	}
	if len(payload) < 1 {
		c.IndentedJSON(400, Message{Status: 0, Payload: ErrorStreamsLen0.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamsMultiControlDelete] [len(payload)] %s", ErrorStreamsLen0.Error())
		return
	}
	var resp = make(map[string]Message)
	var FoundError bool
	for _, key := range payload {
		err := Storage.StreamDelete(key)
		if err != nil {
			log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamsMultiControlDelete] [StreamDelete] stream=%s: %s", key, err.Error())
			resp[key] = Message{Status: 0, Payload: err.Error()}
			FoundError = true
		} else {
			resp[key] = Message{Status: 1, Payload: Success}
		}
	}
	if FoundError {
		c.IndentedJSON(200, Message{Status: 0, Payload: resp})
	} else {
		c.IndentedJSON(200, Message{Status: 1, Payload: resp})
	}
}

// HTTPAPIServerStreamAdd function add new stream
func HTTPAPIServerStreamAdd(c *gin.Context) {
	log.Printf("[INFO] [http_stream] [HTTPAPIServerStreamAdd] regist stream req.: stream=%s", c.Param("uuid"))
	var payload StreamST
	err := c.BindJSON(&payload)
	if err != nil {
		c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamAdd] [BindJSON] stream=%s: %s", c.Param("uuid"), err.Error())
		return
	}
	err = Storage.StreamAdd(c.Param("uuid"), payload)
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamAdd] [StreamAdd] stream=%s: %s", c.Param("uuid"), err.Error())
		return
	}
	c.IndentedJSON(200, Message{Status: 1, Payload: Success})
}

// HTTPAPIServerStreamEdit function edit stream
func HTTPAPIServerStreamEdit(c *gin.Context) {
	log.Printf("[INFO] [http_stream] [HTTPAPIServerStreamEdit] update stream req.: stream=%s", c.Param("uuid"))
	var payload StreamST
	err := c.BindJSON(&payload)
	if err != nil {
		c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamEdit] [BindJSON] stream=%s: %s", c.Param("uuid"), err.Error())
		return
	}
	err = Storage.StreamEdit(c.Param("uuid"), payload)
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamEdit] [StreamEdit] stream=%s: %s", c.Param("uuid"), err.Error())
		return
	}
	c.IndentedJSON(200, Message{Status: 1, Payload: Success})
}

// HTTPAPIServerStreamDelete function delete stream
func HTTPAPIServerStreamDelete(c *gin.Context) {
	log.Printf("[INFO] [http_stream] [HTTPAPIServerStreamDelete] delete stream req.: stream=%s", c.Param("uuid"))
	err := Storage.StreamDelete(c.Param("uuid"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamDelete] [StreamDelete] stream=%s: %s", c.Param("uuid"), err.Error())
		return
	}
	c.IndentedJSON(200, Message{Status: 1, Payload: Success})
}

// HTTPAPIServerStreamDelete function reload stream
func HTTPAPIServerStreamReload(c *gin.Context) {
	err := Storage.StreamReload(c.Param("uuid"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamReload] [StreamReload] stream=%s: %s", c.Param("uuid"), err.Error())
		return
	}
	c.IndentedJSON(200, Message{Status: 1, Payload: Success})
}

// HTTPAPIServerStreamInfo function return stream info struct
func HTTPAPIServerStreamInfo(c *gin.Context) {
	info, err := Storage.StreamInfo(c.Param("uuid"))
	if err != nil {
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_stream] [HTTPAPIServerStreamInfo] [StreamInfo] stream=%s: %s", c.Param("uuid"), err.Error())
		return
	}
	c.IndentedJSON(200, Message{Status: 1, Payload: info})
}
