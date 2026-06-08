package model

import "time"

// OrderModel is the Gorm persistence model for the orders table.
type OrderModel struct {
	ID          string `gorm:"primaryKey;size:64"`
	Name        string `gorm:"size:128;not null"`
	Description string `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (OrderModel) TableName() string {
	return "orders"
}

const orderMongoCollectionName = "orders"

// OrderDocument is the MongoDB document model for the orders collection.
type OrderDocument struct {
	ID          string    `bson:"_id"`
	Name        string    `bson:"name"`
	Description string    `bson:"description"`
	CreatedAt   time.Time `bson:"created_at"`
	UpdatedAt   time.Time `bson:"updated_at"`
}

func (OrderDocument) MongoCollectionName() string {
	return orderMongoCollectionName
}
