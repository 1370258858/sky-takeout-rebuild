package dao

import (
	"context"
	"sky-takeout/microservices/goodsService/global"
	"sky-takeout/microservices/goodsService/internal/model"

	"gorm.io/gorm"
)

type DishDao struct {
	db *gorm.DB
}

func (d *DishDao) List(ctx context.Context, req model.Resquest) ([]model.Dish, error) {
	var dishes []model.Dish
	err := d.db.WithContext(ctx).Model(&dishes).Find(&dishes).Error
	if err != nil {
		global.Log.ErrContext(ctx, "List failed, err: %v", err)
		return dishes, err

	}
	return dishes, nil
}

func NewDishDao(db *gorm.DB) *DishDao {
	return &DishDao{db: db}
}
