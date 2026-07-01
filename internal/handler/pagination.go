package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const maxPageSize = 100

func parsePagination(c *gin.Context, defaultPageSize int) (int, int) {
	page := parsePositiveQueryInt(c, "page", 1)
	pageSize := parsePositiveQueryInt(c, "page_size", defaultPageSize)
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

func parsePositiveQueryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
