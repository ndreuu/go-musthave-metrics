package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTrustedSubnet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		subnet string
		ip     string
		status int
	}{
		{"disabled without header", "", "", http.StatusOK},
		{"disabled with invalid header", "", "not-an-ip", http.StatusOK},
		{"IPv4 inside", "192.168.1.0/24", "192.168.1.42", http.StatusOK},
		{"IPv4 network boundary", "192.168.1.0/24", "192.168.1.0", http.StatusOK},
		{"IPv4 upper boundary", "192.168.1.0/24", "192.168.1.255", http.StatusOK},
		{"IPv4 outside", "192.168.1.0/24", "192.168.2.42", http.StatusForbidden},
		{"CIDR with host bits", "192.168.1.42/24", "192.168.1.1", http.StatusOK},
		{"IPv4 single host", "192.168.1.42/32", "192.168.1.42", http.StatusOK},
		{"IPv4 different host", "192.168.1.42/32", "192.168.1.43", http.StatusForbidden},
		{"IPv6 inside", "2001:db8::/32", "2001:db8:1::42", http.StatusOK},
		{"IPv6 outside", "2001:db8::/32", "2001:db9::42", http.StatusForbidden},
		{"IPv4 mapped IPv6", "192.168.1.0/24", "::ffff:192.168.1.42", http.StatusOK},
		{"address family mismatch", "2001:db8::/32", "192.168.1.42", http.StatusForbidden},
		{"missing header", "192.168.1.0/24", "", http.StatusForbidden},
		{"invalid IP", "192.168.1.0/24", "not-an-ip", http.StatusForbidden},
		{"IP with port", "192.168.1.0/24", "192.168.1.42:1234", http.StatusForbidden},
		{"IP list", "192.168.1.0/24", "192.168.1.42, 192.168.2.42", http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check, err := TrustedSubnet(tt.subnet)
			if err != nil {
				t.Fatal(err)
			}
			called := false
			router := gin.New()
			router.POST("/update", check, func(c *gin.Context) {
				called = true
				c.Status(http.StatusOK)
			})
			request := httptest.NewRequest(http.MethodPost, "/update", nil)
			request.RemoteAddr = "192.168.1.42:1234"
			if tt.ip != "" {
				request.Header.Set("X-Real-IP", tt.ip)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tt.status {
				t.Fatalf("status = %d, want %d", response.Code, tt.status)
			}
			if called != (tt.status == http.StatusOK) {
				t.Fatalf("handler called = %v, status = %d", called, tt.status)
			}
		})
	}
}

func TestTrustedSubnetInvalidCIDR(t *testing.T) {
	for _, cidr := range []string{"not-a-subnet", "192.168.1.0", "192.168.1.0/33", "2001:db8::/129"} {
		t.Run(cidr, func(t *testing.T) {
			check, err := TrustedSubnet(cidr)
			if err == nil || check != nil {
				t.Fatalf("TrustedSubnet(%q) returned middleware %v, error %v", cidr, check != nil, err)
			}
		})
	}
}
