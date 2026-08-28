package gincontextutils

import "github.com/gin-gonic/gin"

const CtxUserIDKey = "user_id"

func CallerID(c *gin.Context) int64 {
	id, _ := c.Get(CtxUserIDKey)

	v, _ := id.(int64)

	return v
}
