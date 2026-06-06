package global

import (
	"sky-takeout/microservices/goodsService/config"

	logger "github.com/Meng-Xin/logger"
	redis "github.com/redis/go-redis/v9"

	"gorm.io/gorm"
)

// 全局变量，供业务层使用
var (
	Config *config.AllConfig // 全局Config
	Log    logger.ILog
	DB     *gorm.DB
	Redis  *redis.Client
)
