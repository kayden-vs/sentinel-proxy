package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type Info struct {
	UserID string `json:"user_id"`
	Method string `json:"method"`
	Role   string `json:"role,omitempty"`
	Raw    string `json:"-"`
}

type Resolver struct {
	jwtSecret []byte
}

func NewResolver(jwtSecret string) *Resolver {
	var secret []byte
	if jwtSecret != "" {
		secret = []byte(jwtSecret)
	}
	return &Resolver{jwtSecret: secret}
}

func (r *Resolver) Resolve(req *http.Request) *Info {
	if info := r.resolveJWT(req); info != nil {
		return info
	}
	if info := r.resolveAPIKey(req); info != nil {
		return info
	}
	return r.resolveFingerprint(req)
}

func (r *Resolver) resolveJWT(req *http.Request) *Info {
	if len(r.jwtSecret) == 0 {
		return nil
	}

	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		return nil
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return nil
	}

	tokenStr := strings.TrimSpace(parts[1])
	if tokenStr == "" {
		return nil
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return r.jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return nil
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return nil
	}

	role := ""
	if rv, ok := claims["role"].(string); ok {
		role = rv
	}

	return &Info{
		UserID: "jwt:" + sub,
		Method: "jwt",
		Role:   role,
		Raw:    sub,
	}
}

func (r *Resolver) resolveAPIKey(req *http.Request) *Info {
	apiKey := req.Header.Get("X-API-Key")
	if apiKey == "" {
		return nil
	}

	hash := sha256Hash(apiKey)
	return &Info{
		UserID: "apikey:" + hash[:16],
		Method: "api_key",
		Raw:    apiKey,
	}
}

func (r *Resolver) resolveFingerprint(req *http.Request) *Info {
	ip := extractClientIP(req)
	ua := req.Header.Get("User-Agent")
	// FIX: Include Accept-Language for stronger fingerprinting.
	// Reduces collision rate when multiple users share the same IP.
	al := strings.ToLower(strings.TrimSpace(req.Header.Get("Accept-Language")))

	raw := ip + "|" + ua + "|" + al
	hash := sha256Hash(raw)

	return &Info{
		UserID: "fp:" + hash[:16],
		Method: "fingerprint",
		Raw:    raw,
	}
}

func extractClientIP(req *http.Request) string {
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}

	if xri := req.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return host
}

func sha256Hash(input string) string {
	h := sha256.New()
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil))
}