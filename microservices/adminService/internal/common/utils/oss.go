package utils

import (
	"context"
	"mime/multipart"
	"sky-takeout/microservices/adminService/global"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

func AliyunOss(ctx context.Context, fileName string, file *multipart.FileHeader) (string, error) {
	config := global.Config.AliOss
	client, err := oss.New(config.EndPoint, config.AccessKeyId, config.AccessKeySecret)
	if err != nil {
		return "", err
	}
	bucket, err := client.Bucket(config.BucketName)
	if err != nil {
		return "", err
	}

	fileData, err := file.Open()
	defer fileData.Close()

	err = bucket.PutObject(fileName, fileData)
	if err != nil {
		return "", err
	}
	imagePath := "https://" + config.BucketName + "." + config.EndPoint + "/" + fileName
	global.Log.InfoContext(ctx, "文件上传到：", imagePath)
	return imagePath, nil
}
