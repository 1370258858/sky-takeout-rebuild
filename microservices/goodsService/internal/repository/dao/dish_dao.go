package dao

import (
	"context"
	"errors"
	"log"
	"sky-takeout/microservices/goodsService/global"
	"sky-takeout/microservices/goodsService/internal/model"

	"gorm.io/gorm"
)

type DishDao struct {
	db *gorm.DB
}

func (d *DishDao) List(ctx context.Context, req model.Resquest) ([]model.Dish, error) {
	var dishes []model.Dish
	tx := d.db.WithContext(ctx).Model(&dishes).Find(&dishes)
	if tx.Error != nil {
		global.Log.ErrContext(ctx, "List failed, err: %v", tx.Error)
		return dishes, tx.Error

	}
	log.Printf("[DB][goods] query list rows=%d", tx.RowsAffected)
	return dishes, nil
}

func (d *DishDao) GetByID(ctx context.Context, id uint64) (*model.Dish, error) {
	var dish model.Dish
	tx := d.db.WithContext(ctx).Model(&model.Dish{}).Where("id = ?", id).Preload("Flavors").Take(&dish)
	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, tx.Error
	}
	log.Printf("[DB][goods] query by id rows=%d goodId=%d", tx.RowsAffected, id)
	return &dish, nil
}

func NewDishDao(db *gorm.DB) *DishDao {
	return &DishDao{db: db}
}
