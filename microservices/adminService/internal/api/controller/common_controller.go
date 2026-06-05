package controller

import (
	"sky-takeout/microservices/adminService/internal/common/retcode"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"sky-takeout/microservices/adminService/global"
	"sky-takeout/microservices/adminService/internal/common/utils"
)

type CommonController struct {
}

func (cc *CommonController) Upload(ctx *gin.Context) {
	// 获取前端传递的图片
	file, err := ctx.FormFile("file")
	if err != nil {
		return
	}
	// 拼接uuid的图片名称
	uuid := uuid.New()
	imageName := uuid.String() + file.Filename
	imagePath, err := utils.AliyunOss(ctx, imageName, file)
	if err != nil {
		global.Log.ErrContext(ctx, "AliyunOss upload failed err=%s", err.Error())
		retcode.Fatal(ctx, err, "")
		return
	}
	retcode.OK(ctx, imagePath)
}
