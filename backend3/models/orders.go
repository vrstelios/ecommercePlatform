package models

type Orders struct {
	Id          string  `json:"id"`
	UserId      string  `json:"userId"`
	TotalAmount float64 `json:"totalAmount"`
	Status      string  `json:"status"`
}

type Payment struct {
	Id            string  `json:"id"`
	OrderId       string  `json:"orderId"`
	UserId        string  `json:"userId"`
	Amount        float64 `json:"amount"`
	PaymentMethod string  `json:"paymentMethod"`
	Status        string  `json:"status"`
}
