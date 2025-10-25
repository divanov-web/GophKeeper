package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	authCookieName = "auth_token"
)

type contextKey string

const UserKey contextKey = "user_id"

// WithAuth добавляет user_id в контекст, если токен валиден
func WithAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(authCookieName)
			if err == nil {
				token, err := jwt.Parse(cookie.Value, func(token *jwt.Token) (interface{}, error) {
					return []byte(secret), nil
				})
				if err == nil && token.Valid {
					if claims, ok := token.Claims.(jwt.MapClaims); ok {
						if userIDFloat, ok := claims["user_id"].(float64); ok {
							userID := int64(userIDFloat)
							ctx := context.WithValue(r.Context(), UserKey, userID)
							r = r.WithContext(ctx)
						}
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SetLoginCookie устанавливает токен с user_id
func SetLoginCookie(w http.ResponseWriter, userID int64, secret string) error {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(365 * 24 * time.Hour).Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    signed,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// GetUserIDFromContext достаёт user_id из контекста
func GetUserIDFromContext(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(UserKey).(int64)
	return userID, ok
}
