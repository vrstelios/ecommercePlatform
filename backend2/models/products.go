package models

type Products struct {
	Id          string  `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Price       float64 `json:"price"`
	CategoryId  string  `json:"categoryId"`
}
