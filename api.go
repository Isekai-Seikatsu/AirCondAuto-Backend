package main

import (
	"github.com/gin-gonic/gin"
)

// CORSAllowAll add CORS allow origin all
func CORSAllowAll() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://localhost:8080")
		c.Next()
	}
}
