package global

import (
	goodrpcv1 "sky-takeout/microservices/rpc/pb/goodsv1"

	logger "github.com/Meng-Xin/logger"
	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Global resources for business layers.
var (
	Log            logger.ILog
	DB             *gorm.DB
	Redis          *redis.Client
	GoodsRPCClient goodrpcv1.GoodsClient
)
