package middleware

import (
	"log/slog"
	"net/http"

	"github.com/casbin/casbin/v2"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func NewEnforcer(db *gorm.DB) (*casbin.Enforcer, error) {
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, err
	}
	enforcer, err := casbin.NewEnforcer("config/casbin_model.conf", adapter)
	if err != nil {
		return nil, err
	}
	if err := enforcer.LoadPolicy(); err != nil {
		return nil, err
	}
	return enforcer, nil
}

func SeedPolicies(e *casbin.Enforcer) {
	policies := [][]string{
		{"enterprise", "/api/v1/enterprise/*", "(GET|POST|PUT|DELETE)"},
		{"carrier", "/api/v1/carrier/*", "(GET|POST|PUT|DELETE)"},
		{"government", "/api/v1/gov/*", "(GET|POST|PUT|DELETE)"},
		{"*", "/api/v1/auth/*", "(GET|POST|PUT)"},
	}
	for _, p := range policies {
		args := make([]interface{}, len(p))
		for i, v := range p {
			args[i] = v
		}
		if added, _ := e.AddPolicy(args...); added {
			slog.Info("casbin policy added", "sub", p[0], "obj", p[1], "act", p[2])
		}
	}
}

func RbacMiddleware(e *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := GetRole(c)
		path := c.Request.URL.Path
		method := c.Request.Method

		allowed, err := e.Enforce(role, path, method)
		if err != nil {
			c.JSON(http.StatusForbidden, gin.H{"code": 10102, "message": "无权限访问"})
			c.Abort()
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"code": 10102, "message": "无权限访问"})
			c.Abort()
			return
		}
		c.Next()
	}
}
