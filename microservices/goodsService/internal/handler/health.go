package handler

import (
	"sky-takeout/microservices/goodsService/common/retcode"

	"github.com/gin-gonic/gin"
)

func Health(c *gin.Context) {
	retcode.OK(c, gin.H{
		"service": "goodsService",
		"status":  "ok",
	})
}
