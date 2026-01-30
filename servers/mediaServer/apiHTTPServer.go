package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// TODO add to next version
// HTTPAPIServerEdit function edit server
func HTTPAPIServerEdit(c *gin.Context) {
	var payload ServerST
	err := c.BindJSON(&payload)
	if err != nil {
		c.IndentedJSON(400, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_server] [HTTPAPIServerEdit] [BindJSON] %s", err.Error())
		return
	}

	log.Println("payLoad Binding.....")

	err = Storage.ServerEdit(payload)

	log.Println("config Editing.....")
	if err != nil {
		log.Println("failed to edit config.....")
		c.IndentedJSON(500, Message{Status: 0, Payload: err.Error()})
		log.Printf("[ERROR] [http_server] [HTTPAPIServerEdit] [ServerEdit] %s", err.Error())
		return
	}
	log.Println("config Edit complete.....")
	c.IndentedJSON(200, Message{Status: 1, Payload: Success})

}

func HTTPAPIServerSettings(c *gin.Context) {
	lang := c.DefaultQuery("lang", "en")
	renderMaintenanceSettings(c, lang, gin.H{})
}

func HTTPAPIServerSettingsUpdate(c *gin.Context) {
	lang := c.DefaultQuery("lang", "en")
	texts := getMaintenanceTexts(lang)

	var form MaintenanceSettingsForm
	if err := c.ShouldBind(&form); err != nil {
		renderMaintenanceSettings(c, lang, gin.H{"error": err.Error()})
		return
	}

	cfg := &Storage.Server.Maintenance
	oldBaseRoot := cfg.BaseRoot

	cfg.RetentionDays = form.RetentionDays
	cfg.RetentionCapacity = form.RetentionCapacity
	cfg.DefaultSafetyFreeSpace = form.DefaultSafetyFreeSpace
	cfg.BaseRoot = form.BaseRoot

	baseRootChanged := oldBaseRoot != cfg.BaseRoot
	if baseRootChanged {
		Storage.setRetentionRoot()
	}

	if err := Storage.SaveConfig(); err != nil {
		renderMaintenanceSettings(c, lang, gin.H{"error": err.Error()})
		return
	}

	var message string
	if baseRootChanged && len(Storage.Recordings) > 0 {
		Storage.AllStreamRestartRecording()
		message = texts["SuccessSavedRestart"]
	} else {
		message = texts["SuccessSaved"]
	}

	renderMaintenanceSettings(c, lang, gin.H{"success": true, "successMessage": message})
}

func renderMaintenanceSettings(c *gin.Context, lang string, extra gin.H) {
	texts := getMaintenanceTexts(lang)
	cfg := Storage.Server.Maintenance
	resolvedBaseRoot := Storage.EffectiveBaseRoot()
	data := gin.H{
		"port":                   Storage.ServerHTTPPort(),
		"streams":                Storage.Streams,
		"version":                time.Now().String(),
		"page":                   "settings",
		"lang":                   lang,
		"texts":                  texts,
		"retentionDays":          cfg.RetentionDays,
		"retentionCapacity":      cfg.RetentionCapacity,
		"baseRoot":               resolvedBaseRoot,
		"defaultSafetyFreeSpace": cfg.DefaultSafetyFreeSpace,
	}

	for k, v := range extra {
		data[k] = v
	}

	c.HTML(http.StatusOK, "settings.tmpl", data)
}
