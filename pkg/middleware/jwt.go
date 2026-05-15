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

// JWT signs tokens and validates bearer authentication for Gin routes.
type JWT struct {
	cfg JWTConfig
}

// DefaultJWTConfig returns non-secret JWT defaults; Secret must come from config.
func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		Issuer:        "xiaolanshu",
		ExpireSeconds: 7200,
	}
}

// NewJWT builds a JWT middleware instance with defaults applied.
func NewJWT(cfg JWTConfig) *JWT {
	defaults := DefaultJWTConfig()
	if cfg.Issuer == "" {
		cfg.Issuer = defaults.Issuer
	}
	if cfg.ExpireSeconds <= 0 {
		cfg.ExpireSeconds = defaults.ExpireSeconds
	}
	return &JWT{cfg: cfg}
}

// GenerateToken signs a JWT for the provided claims using the instance config.
func (j *JWT) GenerateToken(claims JWTClaims) (string, error) {
	if strings.TrimSpace(j.cfg.Secret) == "" {
		return "", errors.New("jwt secret is required")
	}

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, registeredJWTClaims{
		JWTClaims: claims,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.cfg.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(j.cfg.ExpireSeconds) * time.Second)),
		},
	})
	return token.SignedString([]byte(j.cfg.Secret))
}

// Auth validates Authorization: Bearer tokens and stores claims in Gin context.
func (j *JWT) Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(j.cfg.Secret) == "" {
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
			return []byte(j.cfg.Secret), nil
		}, jwt.WithIssuer(j.cfg.Issuer))
		if err != nil || !token.Valid {
			abortUnauthorized(c, "invalid_token", "invalid bearer token")
			return
		}

		c.Set(claimsContextKey, claims.JWTClaims)
		c.Next()
	}
}

// GenerateToken signs a JWT for the provided claims using the configured secret.
func GenerateToken(cfg JWTConfig, claims JWTClaims) (string, error) {
	return NewJWT(cfg).GenerateToken(claims)
}

// JWTAuth validates Authorization: Bearer tokens and stores claims in Gin context.
func JWTAuth(cfg JWTConfig) gin.HandlerFunc {
	return NewJWT(cfg).Auth()
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
