package models

import "time"

type CartItems struct {
	Id        string `json:"id"`
	CartId    string `json:"cartId"`
	ProductId string `json:"productId"`
	Quantity  int    `json:"quantity"`
}

type InventoryItems struct {
	Id            string    `json:"id"`
	ProductId     string    `json:"productId"`
	StockQuantity *int      `json:"stock,omitempty"`
	LastUpdated   time.Time `json:"lastUpdated"`
}
