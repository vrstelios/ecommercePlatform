package api

import (
	"ecommercePlatform/backend1/models"
	"github.com/gin-gonic/gin"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"net/http"
	"time"
)

func PostCartItems(ctx *gin.Context, session *gocql.Session) {
	var item models.CartItems
	if err := ctx.ShouldBindJSON(&item); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if len(item.Id) == 0 {
		uid, err := uuid.NewUUID()
		if err != nil {
			return
		}
		item.Id = uid.String()
	}

	var stock int
	err := session.Query("SELECT stock_quantity FROM inventory_items WHERE product_id = ?", item.ProductId).Scan(&stock)
	if err != nil {
		ctx.JSON(404, gin.H{"error": "Product not found"})
		return
	}

	if stock < item.Quantity {
		ctx.JSON(400, gin.H{"error": "Not enough stock"})
		return
	}

	newStock := stock - item.Quantity
	query := "UPDATE inventory_items SET stock_quantity = ? WHERE product_id = ? IF stock_quantity = ?"

	mapping := make(map[string]interface{})
	applied, err := session.Query(query, newStock, item.ProductId, stock).MapScanCAS(mapping)
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if !applied {
		ctx.JSON(409, gin.H{"error": "Race condition! Stock changed, please try again."})
		return
	}

	err = session.Query("INSERT INTO cart_items (id, cart_id, product_id, quantity) VALUES (?, ?, ?, ?) USING TTL 900", item.Id, item.CartId, item.ProductId, item.Quantity).Exec()
	if err != nil {
		ctx.JSON(500, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(200, gin.H{"message": "Success! Stock updated and reserved."})
}

func GetCartItems(ctx *gin.Context, session *gocql.Session) {
	id := ctx.Param("id")
	if len(id) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	var items []models.CartItems

	query := "SELECT id, cart_id, product_id, quantity FROM cart_items WHERE cart_id = ?"
	iter := session.Query(query, id).Iter()

	var pId, cartId, productId string
	var qty int
	for iter.Scan(&pId, &cartId, &productId, &qty) {
		items = append(items, models.CartItems{
			Id:        pId,
			CartId:    cartId,
			ProductId: productId,
			Quantity:  qty,
		})
	}

	if err := iter.Close(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Query failed"})
		return
	}

	ctx.JSON(http.StatusOK, items)
}

func PostInventory(ctx *gin.Context, session *gocql.Session) {
	var item models.InventoryItems
	if err := ctx.ShouldBindJSON(&item); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if len(item.Id) == 0 {
		uid, err := uuid.NewUUID()
		if err != nil {
			return
		}
		item.Id = uid.String()
	}

	now := time.Now()
	query := "INSERT INTO inventory_items (product_id, id, stock_quantity, last_updated) VALUES (?, ?, ?, ?)"
	if err := session.Query(query, item.ProductId, item.Id, item.StockQuantity, now).Exec(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save to Cassandra"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Inventory updated in Cassandra"})
}

func GetInventory(ctx *gin.Context, session *gocql.Session) {
	id := ctx.Param("id")
	var stock int

	query := "SELECT stock_quantity FROM inventory_items WHERE product_id = ? LIMIT 1"
	if err := session.Query(query, id).Scan(&stock); err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Product not found in Cassandra"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"productId": id, "stock": stock})
}
