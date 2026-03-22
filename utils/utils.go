package utils

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func GetLoggerWithTrace(ctx *gin.Context, logger *zap.Logger) *zap.Logger {
	traceId := ctx.GetHeader("X-Correlation-Id")
	if len(traceId) == 0 {
		traceId = "internal-gen-" + uuid.NewString()
	}
	return logger.With(zap.String("trace_id", traceId))
}
