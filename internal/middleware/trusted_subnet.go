package middleware

import (
	"fmt"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
)

// TrustedSubnet проверяет IP из X-Real-IP перед обработкой метрик.
// Пустая подсеть отключает проверку; непустое значение должно быть CIDR.
func TrustedSubnet(cidr string) (gin.HandlerFunc, error) {
	var subnet *net.IPNet
	if cidr != "" {
		var err error
		_, subnet, err = net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted subnet %q: %w", cidr, err)
		}
	}

	return func(c *gin.Context) {
		if subnet != nil {
			ip := net.ParseIP(c.GetHeader("X-Real-IP"))
			if ip == nil || !subnet.Contains(ip) {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}
		c.Next()
	}, nil
}
