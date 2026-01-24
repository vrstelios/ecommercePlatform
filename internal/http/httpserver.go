package http

import "github.com/gin-gonic/gin"

func SetupRoutes(router *gin.Engine) {

	routerEndpoints := router.Group("/api")

	routerEndpoints.Use()
	{
		routerEndpoints.GET("/px")
	}
}
