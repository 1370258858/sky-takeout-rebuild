package dao

import (
	"context"
	"sky-takeout/microservices/goodsService/common/e"
	"sky-takeout/microservices/goodsService/common/retcode"
	"sky-takeout/microservices/goodsService/global"
	"sky-takeout/microservices/goodsService/internal/model"

	"gorm.io/gorm"
)

type DishDao struct {
	db *gorm.DB
}

func (d *DishDao) List(ctx context.Context, dish model.Dish) error {
	err := d.db.WithContext(ctx).Model(&dish).Find(&dish).Error
	if err != nil {
		global.Log.ErrContext(ctx, "List failed, err: %v", err)
		return retcode.NewError(e.MysqlERR, "List category failed")
	}
	return nil
}

func NewDishDao(db *gorm.DB) *DishDao {
	return &DishDao{db: db}
}
