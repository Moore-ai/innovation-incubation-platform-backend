package response

import (
	"fmt"
	"net/http"

	"innovation-incubation-platform-backend/pkg/errcode"

	"github.com/gin-gonic/gin"
)

type PageData struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

var errHTTPStatus = map[int]int{
	0:     200,
	10001: http.StatusBadRequest,
	10002: http.StatusNotFound,
	10003: http.StatusConflict,
	10101: http.StatusUnauthorized,
	10102: http.StatusForbidden,
	10103: http.StatusTooManyRequests,
	10201: http.StatusConflict,
	10202: http.StatusConflict,
	10301: http.StatusBadGateway,
	10302: http.StatusGatewayTimeout,
	50000: http.StatusInternalServerError,
}

func errToHTTP(code int) int {
	if status, ok := errHTTPStatus[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    data,
	})
}

func Created(c *gin.Context, data any, location string) {
	if location != "" {
		c.Header("Location", location)
	}
	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "success",
		"data":    data,
	})
}

func SuccessPage(c *gin.Context, list any, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    PageData{List: list, Total: total, Page: page, PageSize: pageSize},
		"links": func() gin.H {
			links := gin.H{"self": fmt.Sprintf("%s?page=%d&page_size=%d", c.Request.URL.Path, page, pageSize)}
			if int64(page)*int64(pageSize) < total {
				links["next"] = fmt.Sprintf("%s?page=%d&page_size=%d", c.Request.URL.Path, page+1, pageSize)
			}
			return links
		}(),
	})
}

func Error(c *gin.Context, err error) {
	if appErr, ok := err.(*errcode.AppError); ok {
		c.JSON(errToHTTP(appErr.Code), gin.H{
			"code":    appErr.Code,
			"message": appErr.Message,
		})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"code":    50000,
		"message": err.Error(),
	})
}
