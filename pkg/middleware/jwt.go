package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const claimsContextKey = "jwt_claims"

// JWTConfig controls token signing and validation.
type JWTConfig struct {
	Secret        string `mapstructure:"secret" yaml:"secret"`
	Issuer        string `mapstructure:"issuer" yaml:"issuer"`
	ExpireSeconds int64  `mapstructure:"expire_seconds" yaml:"expire_seconds"`
}

// JWTClaims is the business payload stored in signed tokens.
type JWTClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

type registeredJWTClaims struct {
	JWTClaims
	jwt.RegisteredClaims
}

// DefaultJWTConfig returns non-secret JWT defaults; Secret must come from config.
func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		Issuer:        "xiaolanshu",
		ExpireSeconds: 7200,
	}
}

// GenerateToken signs a JWT for the provided claims using the configured secret.
func GenerateToken(cfg JWTConfig, claims JWTClaims) (string, error) {
	if strings.TrimSpace(cfg.Secret) == "" {
		return "", errors.New("jwt secret is required")
	}
	if cfg.Issuer == "" {
		cfg.Issuer = DefaultJWTConfig().Issuer
	}
	if cfg.ExpireSeconds <= 0 {
		cfg.ExpireSeconds = DefaultJWTConfig().ExpireSeconds
	}

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, registeredJWTClaims{
		JWTClaims: claims,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(cfg.ExpireSeconds) * time.Second)),
		},
	})
	return token.SignedString([]byte(cfg.Secret))
}

// JWTAuth validates Authorization: Bearer tokens and stores claims in Gin context.
func JWTAuth(cfg JWTConfig) gin.HandlerFunc {
	if cfg.Issuer == "" {
		cfg.Issuer = DefaultJWTConfig().Issuer
	}

	return func(c *gin.Context) {
		if strings.TrimSpace(cfg.Secret) == "" {
			abortUnauthorized(c, "jwt_secret_missing", "jwt secret is not configured")
			return
		}
		tokenText := bearerToken(c.GetHeader("Authorization"))
		if tokenText == "" {
			abortUnauthorized(c, "missing_token", "missing bearer token")
			return
		}

		claims := &registeredJWTClaims{}
		token, err := jwt.ParseWithClaims(tokenText, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrTokenSignatureInvalid
			}
			return []byte(cfg.Secret), nil
		}, jwt.WithIssuer(cfg.Issuer))
		if err != nil || !token.Valid {
			abortUnauthorized(c, "invalid_token", "invalid bearer token")
			return
		}

		c.Set(claimsContextKey, claims.JWTClaims)
		c.Next()
	}
}

// ClaimsFromContext returns JWT claims parsed by JWTAuth.
func ClaimsFromContext(c *gin.Context) JWTClaims {
	value, ok := c.Get(claimsContextKey)
	if !ok {
		return JWTClaims{}
	}
	claims, _ := value.(JWTClaims)
	return claims
}

func bearerToken(header string) string {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func abortUnauthorized(c *gin.Context, code string, message string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
