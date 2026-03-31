package auth

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rushteam/dagflow/internal/infrastructure/database/gen"
)

type contextKey string

const userIDKey contextKey = "user_id"

type JWTManager struct {
	secret     []byte
	expiration time.Duration
	queries    *gen.Queries
}

func NewJWTManager(secret string, expiration time.Duration, db gen.DBTX) *JWTManager {
	return &JWTManager{
		secret:     []byte(secret),
		expiration: expiration,
		queries:    gen.New(db),
	}
}

func (m *JWTManager) GenerateToken(userID int64, username string) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"exp":      time.Now().Add(m.expiration).Unix(),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JWTManager) ParseToken(tokenString string) (int64, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return 0, fmt.Errorf("invalid token")
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid user_id in token")
	}

	return int64(userIDFloat), nil
}

// Middleware 验证请求身份，支持两种模式：
//   - JWT: Authorization: Bearer <jwt>
//   - API Token: Authorization: Bearer tk_<token>
func (m *JWTManager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		if tokenStr == header {
			http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
			return
		}

		// API Token: dsh_ 前缀
		if strings.HasPrefix(tokenStr, "tk_") {
			userID, err := m.verifyAPIToken(r.Context(), tokenStr)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired api token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// JWT
		userID, err := m.ParseToken(tokenStr)
		if err != nil {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *JWTManager) verifyAPIToken(ctx context.Context, raw string) (int64, error) {
	hash := HashToken(raw)
	token, err := m.queries.GetAPITokenByHash(ctx, hash)
	if err != nil {
		return 0, fmt.Errorf("token not found")
	}
	if token.ExpiresAt.Valid && token.ExpiresAt.Time.Before(time.Now()) {
		return 0, fmt.Errorf("token expired")
	}
	go func() {
		_ = m.queries.TouchAPIToken(context.Background(), token.ID)
	}()
	return token.CreatedBy, nil
}

// HashToken 返回 API token 原文的 SHA-256 hex 摘要。
func HashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey).(int64)
	return id, ok
}

// GenerateAPITokenRaw 生成一个随机 API token 原文。
func GenerateAPITokenRaw() (string, error) {
	b := make([]byte, 24)
	if _, err := cryptorand.Read(b); err != nil {
		return "", err
	}
	return "tk_" + hex.EncodeToString(b), nil
}
