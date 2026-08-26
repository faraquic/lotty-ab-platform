package auth_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/faraquic/lotty-ab-platform/pkg/auth"
)

// ensure JWTManager satisfies the Tokenizer contract
var _ auth.Tokenizer = (*auth.JWTManager)(nil)

const testSecret = "test-secret-key"

func newManager(ttl time.Duration) *auth.JWTManager {
	return auth.NewJWTManager(testSecret, ttl)
}

// signToken builds a raw JWT with the given claims using HS256 (or the
// supplied signing method) and the test secret.
func signToken(t *testing.T, method jwt.SigningMethod, secret string, claims jwt.Claims) string {
	t.Helper()

	tok := jwt.NewWithClaims(method, claims)
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return signed
}

func TestGenerateToken(t *testing.T) {
	t.Run("returns token and future expiry", func(t *testing.T) {
		m := newManager(time.Hour)
		before := time.Now()

		token, exp, err := m.GenerateToken(42, "admin")
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if token == "" {
			t.Fatal("token is empty")
		}

		// expiry should sit roughly at now + ttl
		delta := exp.Sub(before)
		if delta < 59*time.Minute || delta > 61*time.Minute {
			t.Errorf("expiry delta = %v, want ~1h", delta)
		}
		if !exp.After(before) {
			t.Errorf("expiry %v is not after %v", exp, before)
		}
	})

	t.Run("round-trips user id across values", func(t *testing.T) {
		m := newManager(time.Hour)

		for _, id := range []int64{1, 99, 1234567890} {
			token, _, err := m.GenerateToken(id, "viewer")
			if err != nil {
				t.Fatalf("generate %d: %v", id, err)
			}

			got, err := m.ParseToken(token)
			if err != nil {
				t.Fatalf("parse %d: %v", id, err)
			}
			if got != id {
				t.Errorf("parsed user id = %d, want %d", got, id)
			}
		}
	})

	t.Run("embeds role in claims", func(t *testing.T) {
		m := newManager(time.Hour)
		token, _, err := m.GenerateToken(7, "approver")
		if err != nil {
			t.Fatal(err)
		}

		parsed, err := jwt.Parse(token, func(*jwt.Token) (any, error) { return []byte(testSecret), nil })
		if err != nil {
			t.Fatal(err)
		}
		claims, ok := parsed.Claims.(jwt.MapClaims)
		if !ok {
			t.Fatalf("claims type = %T, want jwt.MapClaims", parsed.Claims)
		}
		if claims["role"] != "approver" {
			t.Errorf("role claim = %v, want approver", claims["role"])
		}
	})

	t.Run("zero user id yields token rejected on parse", func(t *testing.T) {
		m := newManager(time.Hour)
		token, _, err := m.GenerateToken(0, "admin")
		if err != nil {
			t.Fatal(err)
		}

		if _, err := m.ParseToken(token); !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("err = %v, want ErrInvalidToken", err)
		}
	})
}

func TestParseToken(t *testing.T) {
	t.Run("valid token", func(t *testing.T) {
		m := newManager(time.Hour)
		token, _, err := m.GenerateToken(123, "admin")
		if err != nil {
			t.Fatal(err)
		}

		id, err := m.ParseToken(token)
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if id != 123 {
			t.Errorf("id = %d, want 123", id)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		// negative ttl makes GenerateToken produce an already-expired token
		m := newManager(-time.Hour)
		token, _, err := m.GenerateToken(1, "admin")
		if err != nil {
			t.Fatal(err)
		}

		_, err = m.ParseToken(token)
		if !errors.Is(err, auth.ErrExpiredToken) {
			t.Errorf("err = %v, want ErrExpiredToken", err)
		}
	})

	t.Run("empty token", func(t *testing.T) {
		m := newManager(time.Hour)
		_, err := m.ParseToken("")
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("err = %v, want ErrInvalidToken", err)
		}
	})

	t.Run("garbage token", func(t *testing.T) {
		m := newManager(time.Hour)
		_, err := m.ParseToken("not-a-jwt")
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("err = %v, want ErrInvalidToken", err)
		}
	})

	t.Run("wrong secret", func(t *testing.T) {
		m := newManager(time.Hour)
		// token signed by a manager with a different secret
		other := auth.NewJWTManager("different-secret", time.Hour)
		token, _, err := other.GenerateToken(5, "admin")
		if err != nil {
			t.Fatal(err)
		}

		_, err = m.ParseToken(token)
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("err = %v, want ErrInvalidToken", err)
		}
	})

	t.Run("tampered signature", func(t *testing.T) {
		m := newManager(time.Hour)
		token, _, err := m.GenerateToken(9, "admin")
		if err != nil {
			t.Fatal(err)
		}

		tampered := token + "x"
		_, err = m.ParseToken(tampered)
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("err = %v, want ErrInvalidToken", err)
		}
	})

	t.Run("unexpected signing method (none)", func(t *testing.T) {
		// forge an unsigned "none" token; the keyfunc must reject it
		noneTok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
			"sub": "1",
			"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
		})
		noneToken, err := noneTok.SignedString(jwt.UnsafeAllowNoneSignatureType)
		if err != nil {
			t.Fatalf("sign none token: %v", err)
		}

		m := newManager(time.Hour)
		_, err = m.ParseToken(noneToken)
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("err = %v, want ErrInvalidToken", err)
		}
	})

	t.Run("non-numeric subject", func(t *testing.T) {
		bad := signToken(t, jwt.SigningMethodHS256, testSecret, jwt.MapClaims{
			"sub": "abc",
			"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
		})

		m := newManager(time.Hour)
		_, err := m.ParseToken(bad)
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("err = %v, want ErrInvalidToken", err)
		}
	})

	t.Run("negative subject", func(t *testing.T) {
		bad := signToken(t, jwt.SigningMethodHS256, testSecret, jwt.MapClaims{
			"sub": "-5",
			"exp": jwt.NewNumericDate(time.Now().Add(time.Hour)),
		})

		m := newManager(time.Hour)
		_, err := m.ParseToken(bad)
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("err = %v, want ErrInvalidToken", err)
		}
	})

	t.Run("missing expiry still valid until server clock", func(t *testing.T) {
		// a token without exp is considered valid by jwt-go; ensure we
		// at least parse the subject correctly (no exp -> not expired)
		noExp := signToken(t, jwt.SigningMethodHS256, testSecret, jwt.MapClaims{
			"sub": "77",
		})

		m := newManager(time.Hour)
		id, err := m.ParseToken(noExp)
		if err != nil {
			t.Fatalf("err = %v, want nil (no exp is allowed)", err)
		}
		if id != 77 {
			t.Errorf("id = %d, want 77", id)
		}
	})

	t.Run("invalid base64 segment", func(t *testing.T) {
		m := newManager(time.Hour)
		// header.payload has a bogus (non-base64) signature segment
		bogus := strings.Join([]string{
			"eyJhbGciOiJIUzI1NiJ9",
			"eyJzdWIiOiIxIn0",
			"!!!not-base64!!!",
		}, ".")
		_, err := m.ParseToken(bogus)
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("err = %v, want ErrInvalidToken", err)
		}
	})
}
