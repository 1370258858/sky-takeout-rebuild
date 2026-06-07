package global

import (
	goodsv1 "sky-takeout/microservices/rpc/pb/goodsv1"

	redis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	DB             *gorm.DB
	Redis          *redis.Client
	GoodsRPCClient goodsv1.GoodsClient
)
