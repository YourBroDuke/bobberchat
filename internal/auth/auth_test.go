package auth

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestHashSecretArgon2idFormat(t *testing.T) {
	hash, err := hashSecretArgon2id("super-secret")
	if err != nil {
		t.Fatalf("hashSecretArgon2id returned error: %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("expected argon2id prefix, got %q", hash)
	}

	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		t.Fatalf("expected 6 parts in encoded hash, got %d: %q", len(parts), hash)
	}
	if parts[2] != "v=19" {
		t.Fatalf("expected version part v=19, got %q", parts[2])
	}
	if !strings.Contains(parts[3], "m=") || !strings.Contains(parts[3], "t=") || !strings.Contains(parts[3], "p=") {
		t.Fatalf("expected argon2 params in part 4, got %q", parts[3])
	}
}

func TestVerifySecretArgon2id(t *testing.T) {
	hash, err := hashSecretArgon2id("correct-secret")
	if err != nil {
		t.Fatalf("hashSecretArgon2id returned error: %v", err)
	}

	tests := []struct {
		name   string
		secret string
		want   bool
	}{
		{name: "correct secret", secret: "correct-secret", want: true},
		{name: "wrong secret", secret: "wrong-secret", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := verifySecretArgon2id(hash, tt.secret)
			if got != tt.want {
				t.Fatalf("verifySecretArgon2id() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHashVerifyRoundTrip(t *testing.T) {
	secret := "roundtrip-secret"
	hash, err := hashSecretArgon2id(secret)
	if err != nil {
		t.Fatalf("hashSecretArgon2id returned error: %v", err)
	}

	if !verifySecretArgon2id(hash, secret) {
		t.Fatal("expected verifySecretArgon2id to succeed for matching secret")
	}
}

func TestDecodeArgon2Hash(t *testing.T) {
	validHash, err := hashSecretArgon2id("decode-me")
	if err != nil {
		t.Fatalf("hashSecretArgon2id returned error: %v", err)
	}

	tests := []struct {
		name       string
		encoded    string
		wantErr    bool
		assertGood bool
	}{
		{name: "valid", encoded: validHash, wantErr: false, assertGood: true},
		{name: "invalid part count", encoded: "$argon2id$v=19$m=65536,t=3,p=2$abc", wantErr: true},
		{name: "wrong algorithm", encoded: strings.Replace(validHash, "$argon2id$", "$argon2i$", 1), wantErr: true},
		{name: "incompatible version", encoded: strings.Replace(validHash, "$v=19$", "$v=18$", 1), wantErr: true},
		{name: "invalid params", encoded: strings.Replace(validHash, "m=65536,t=3,p=2", "m=65536,t=3", 1), wantErr: true},
		{name: "invalid memory", encoded: strings.Replace(validHash, "m=65536,t=3,p=2", "m=bad,t=3,p=2", 1), wantErr: true},
		{name: "invalid salt", encoded: validHash[:strings.LastIndex(validHash, "$")] + "$***", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memory, iterations, parallelism, salt, hash, err := decodeArgon2Hash(tt.encoded)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("decodeArgon2Hash returned error: %v", err)
			}
			if tt.assertGood {
				if memory == 0 || iterations == 0 || parallelism == 0 {
					t.Fatalf("expected non-zero params, got memory=%d iterations=%d parallelism=%d", memory, iterations, parallelism)
				}
				if len(salt) == 0 || len(hash) == 0 {
					t.Fatalf("expected non-empty salt/hash, got salt=%d hash=%d", len(salt), len(hash))
				}
			}
		})
	}
}

func TestGenerateSecret(t *testing.T) {
	const n = 32
	first, err := generateSecret(n)
	if err != nil {
		t.Fatalf("generateSecret first call error: %v", err)
	}
	second, err := generateSecret(n)
	if err != nil {
		t.Fatalf("generateSecret second call error: %v", err)
	}

	if first == second {
		t.Fatal("expected generated secrets to differ")
	}

	expectedLen := base64.RawURLEncoding.EncodedLen(n)
	if len(first) != expectedLen {
		t.Fatalf("expected first secret length %d, got %d", expectedLen, len(first))
	}
	if len(second) != expectedLen {
		t.Fatalf("expected second secret length %d, got %d", expectedLen, len(second))
	}
}

func TestValidateJWTRoundTrip(t *testing.T) {
	svc := &Service{jwtSecret: []byte("test-secret")}

	claims := jwtTokenClaims{
		UserID:   "user-1",
		TenantID: "tenant-1",
		Role:     "member",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user:user-1",
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(svc.jwtSecret)
	if err != nil {
		t.Fatalf("signed string error: %v", err)
	}

	validated, err := svc.ValidateJWT(tokenStr)
	if err != nil {
		t.Fatalf("ValidateJWT returned error: %v", err)
	}

	if validated.UserID != claims.UserID || validated.TenantID != claims.TenantID || validated.Role != claims.Role {
		t.Fatalf("unexpected claims: got %+v", validated)
	}
}

func TestValidateJWTRejectsInvalidTokens(t *testing.T) {
	svc := &Service{jwtSecret: []byte("test-secret")}
	now := time.Now().UTC()

	goodClaims := jwtTokenClaims{
		UserID:   "user-1",
		TenantID: "tenant-1",
		Role:     "member",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}

	goodToken := jwt.NewWithClaims(jwt.SigningMethodHS256, goodClaims)
	goodTokenStr, err := goodToken.SignedString(svc.jwtSecret)
	if err != nil {
		t.Fatalf("failed to sign baseline token: %v", err)
	}

	expiredClaims := jwtTokenClaims{
		UserID:   "user-1",
		TenantID: "tenant-1",
		Role:     "member",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
		},
	}
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	expiredTokenStr, err := expiredToken.SignedString(svc.jwtSecret)
	if err != nil {
		t.Fatalf("failed to sign expired token: %v", err)
	}

	wrongMethodToken := jwt.NewWithClaims(jwt.SigningMethodHS384, goodClaims)
	wrongMethodTokenStr, err := wrongMethodToken.SignedString(svc.jwtSecret)
	if err != nil {
		t.Fatalf("failed to sign wrong-method token: %v", err)
	}

	tamperedTokenStr := tamperToken(goodTokenStr)

	tests := []struct {
		name  string
		token string
	}{
		{name: "expired token", token: expiredTokenStr},
		{name: "wrong signing method", token: wrongMethodTokenStr},
		{name: "tampered token", token: tamperedTokenStr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := svc.ValidateJWT(tt.token)
			if err == nil {
				t.Fatalf("expected error for invalid token, got claims=%+v", claims)
			}
		})
	}
}

func tamperToken(token string) string {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 || len(parts[2]) < 4 {
		return token
	}
	sig := []byte(parts[2])
	for i := 0; i < 4; i++ {
		if sig[i] == 'A' {
			sig[i] = 'B'
		} else {
			sig[i] = 'A'
		}
	}
	parts[2] = string(sig)
	return strings.Join(parts, ".")
}
