package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CORSConfig controls the gateway cross-origin policy.
type CORSConfig struct {
	AllowOrigins     []string      `mapstructure:"allow_origins" yaml:"allow_origins"`
	AllowMethods     []string      `mapstructure:"allow_methods" yaml:"allow_methods"`
	AllowHeaders     []string      `mapstructure:"allow_headers" yaml:"allow_headers"`
	ExposeHeaders    []string      `mapstructure:"expose_headers" yaml:"expose_headers"`
	AllowCredentials bool          `mapstructure:"allow_credentials" yaml:"allow_credentials"`
	MaxAge           time.Duration `mapstructure:"max_age" yaml:"max_age"`
}

// DefaultCORSConfig returns permissive local-development CORS defaults.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins:  []string{"*"},
		AllowMethods:  []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders:  []string{"Origin", "Content-Type", "Authorization", HeaderRequestID},
		ExposeHeaders: []string{HeaderRequestID},
		MaxAge:        12 * time.Hour,
	}
}

// CORS applies the configured cross-origin policy and handles preflight requests.
func CORS(cfg CORSConfig) gin.HandlerFunc {
	cfg = fillCORSDefaults(cfg)

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}
		if !originAllowed(origin, cfg.AllowOrigins) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		if allowAllOrigins(cfg.AllowOrigins) && !cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Methods", strings.Join(cfg.AllowMethods, ", "))
		c.Header("Access-Control-Allow-Headers", strings.Join(cfg.AllowHeaders, ", "))
		c.Header("Access-Control-Expose-Headers", strings.Join(cfg.ExposeHeaders, ", "))
		c.Header("Access-Control-Max-Age", strconv.Itoa(int(cfg.MaxAge.Seconds())))
		if cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func fillCORSDefaults(cfg CORSConfig) CORSConfig {
	defaults := DefaultCORSConfig()
	if len(cfg.AllowOrigins) == 0 {
		cfg.AllowOrigins = defaults.AllowOrigins
	}
	if len(cfg.AllowMethods) == 0 {
		cfg.AllowMethods = defaults.AllowMethods
	}
	if len(cfg.AllowHeaders) == 0 {
		cfg.AllowHeaders = defaults.AllowHeaders
	}
	if len(cfg.ExposeHeaders) == 0 {
		cfg.ExposeHeaders = defaults.ExposeHeaders
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = defaults.MaxAge
	}
	return cfg
}

func originAllowed(origin string, allowOrigins []string) bool {
	for _, allowed := range allowOrigins {
		if allowed == "*" || strings.EqualFold(allowed, origin) {
			return true
		}
	}
	return false
}

func allowAllOrigins(allowOrigins []string) bool {
	for _, allowed := range allowOrigins {
		if allowed == "*" {
			return true
		}
	}
	return false
}
