package response

import (
	"net/http"

	"innovation-incubation-platform-backend/pkg/errcode"
	"github.com/gin-gonic/gin"
)

type PageData struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    data,
	})
}

func SuccessPage(c *gin.Context, list interface{}, total int64, page, pageSize int) {
	Success(c, PageData{List: list, Total: total, Page: page, PageSize: pageSize})
}

func Error(c *gin.Context, err error) {
	if appErr, ok := err.(*errcode.AppError); ok {
		c.JSON(http.StatusOK, gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    50000,
		"message": err.Error(),
	})
}
