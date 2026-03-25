package api

import (
	"bytes"
	"context"
	"ecommercePlatform/backend2/models"
	pb "ecommercePlatform/backend2/proto"
	"encoding/json"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"log"
	"net/http"
	"strconv"
)

func PostProductsElastic(ctx *gin.Context, es *elasticsearch.Client, db *pgx.Conn) {
	prod := models.Products{}
	err := ctx.ShouldBindJSON(&prod)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if len(prod.Id) == 0 {
		uid, err := uuid.NewUUID()
		if err != nil {
			return
		}
		prod.Id = uid.String()
	}

	masterSql := `INSERT INTO products (id, name, description, price) VALUES ($1, $2, $3, $4)`
	_, err = db.Exec(context.Background(), masterSql, prod.Id, prod.Name, prod.Description, prod.Price)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save product to database"})
		return
	}

	data, _ := json.Marshal(prod)
	res, err := es.Index(
		"products",
		bytes.NewReader(data),
		es.Index.WithDocumentID(prod.Id),
		es.Index.WithRefresh("true"),
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to index product"})
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Elasticsearch error"})
		return
	}

	ctx.JSON(http.StatusOK, prod)
}

func GetProduct(ctx *gin.Context, es *elasticsearch.Client) {
	id := ctx.Query("id")
	if len(id) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	res, err := es.Get("products", id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Elasticsearch error"})
		return
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}

	var result map[string]interface{}
	json.NewDecoder(res.Body).Decode(&result)

	ctx.JSON(http.StatusOK, result["_source"])
}

type ESResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			Source models.Products `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

func GetProductsElastic(ctx *gin.Context, es *elasticsearch.Client) {
	var err error

	name := ctx.Query("search")
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	from := (page - 1) * limit

	query := map[string]interface{}{
		"from": from,
		"size": limit,
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
	}
	if len(name) != 0 {
		query = map[string]interface{}{
			"query": map[string]interface{}{
				"match": map[string]interface{}{
					"name": name,
				},
			},
		}
	}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(query)

	res, err := es.Search(
		es.Search.WithContext(ctx),
		es.Search.WithIndex("products"),
		es.Search.WithBody(&buf),
		es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "ES Search Error"})
		return
	}
	defer res.Body.Close()

	var esRes ESResponse
	if err := json.NewDecoder(res.Body).Decode(&esRes); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error parsing ES response"})
		return
	}

	var product []models.Products
	for _, hit := range esRes.Hits.Hits {
		product = append(product, hit.Source)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"products": product,
		"total":    esRes.Hits.Total.Value,
		"page":     page,
		"limit":    limit,
	})
}

type ProductServer struct {
	pb.UnimplementedProductServiceServer
	Es *elasticsearch.Client
}

func (s *ProductServer) SearchProducts(ctx context.Context, req *pb.SearchRequest) (*pb.SearchListResponse, error) {
	log.Printf("gRPC Search Request: %s", req.GetQuery())
	name := req.GetQuery()
	page := req.GetPage()
	limit := req.GetLimit()

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	from := (page - 1) * limit

	query := map[string]interface{}{
		"from": from,
		"size": limit,
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
	}
	if len(name) != 0 {
		query = map[string]interface{}{
			"query": map[string]interface{}{
				"match": map[string]interface{}{
					"name": name,
				},
			},
		}
	}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(query)

	res, err := s.Es.Search(
		s.Es.Search.WithContext(ctx),
		s.Es.Search.WithIndex("products"),
		s.Es.Search.WithBody(&buf),
		s.Es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var esRes ESResponse
	json.NewDecoder(res.Body).Decode(&esRes)

	var product []models.Products
	for _, hit := range esRes.Hits.Hits {
		product = append(product, hit.Source)
	}

	var grpcProducts []*pb.ProductResponse
	for _, hit := range esRes.Hits.Hits {
		grpcProducts = append(grpcProducts, &pb.ProductResponse{
			Id:          hit.Source.Id,
			Name:        hit.Source.Name,
			Price:       hit.Source.Price,
			Description: *hit.Source.Description,
		})
	}

	return &pb.SearchListResponse{Products: grpcProducts, Total: int32(esRes.Hits.Total.Value)}, nil
}
