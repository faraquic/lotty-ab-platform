package auth

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	jwt "github.com/appleboy/gin-jwt/v3"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/services/panel/internal/domain/users"
)

const (
	Realm       = "labp-panel"
	IdentityKey = "id"

	requiredRolesKey = "required_roles"
	sessionRevoked   = "session_revoked"
)

type Middleware struct {
	middleware *jwt.GinJWTMiddleware
	svc        *Service
	log        *zap.Logger
}

func NewMiddleware(cfg *config.Config, svc *Service, log *zap.Logger) (*Middleware, error) {
	middleware, err := jwt.New(&jwt.GinJWTMiddleware{
		Realm:         Realm,
		Key:           []byte(cfg.Auth.JWT.SecretKey),
		Timeout:       cfg.Auth.JWT.TTL,
		IdentityKey:   IdentityKey,
		TokenLookup:   "header: Authorization: Bearer",
		TokenHeadName: "Bearer",
		TimeFunc:      time.Now,

		// tokens are minted by our /login handler (sub = user id),
		// so only validation is delegated to gin-jwt
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			sub, _ := claims["sub"].(string)
			id, _ := strconv.ParseInt(sub, 10, 64)

			return id
		},

		// runs after the signature and exp are verified;
		// enforces both the Redis session (revocation) and per-group roles
		Authorizer: func(c *gin.Context, identity interface{}) bool {
			id, _ := identity.(int64)
			token := bearerToken(c.Request.Header.Get("Authorization"))

			if id < 1 || !svc.SessionActive(c.Request.Context(), token, id) {
				c.Set(sessionRevoked, true)
				return false
			}

			c.Set(users.CtxUserIDKey, id)

			raw, exists := c.Get(requiredRolesKey)
			if !exists {
				return true // authenticated-only group, any role
			}

			required, _ := raw.(map[users.Role]struct{})
			role, _ := jwt.ExtractClaims(c)["role"].(string)
			_, allowed := required[users.Role(role)]

			return allowed
		},

		// gin-jwt calls this with 401 for auth failures and 403 when
		// Authorizer returns false; distinguish revoked from forbidden
		Unauthorized: func(c *gin.Context, code int, message string) {
			errCode := "UNAUTHORIZED"

			if code == http.StatusForbidden {
				if revoked, _ := c.Get(sessionRevoked); revoked == true {
					code = http.StatusUnauthorized
					message = "token revoked"
				} else {
					errCode = "FORBIDDEN"
				}
			}

			api.Error(c.Writer, code, errCode, message)
		},
	})
	if err != nil {
		return nil, err
	}

	return &Middleware{middleware: middleware, svc: svc, log: log}, nil
}

// Handler verifies the Bearer token via gin-jwt (signature, expiry,
// revocation, role) before the group's routes run.
// Tokens whose role is not in requiredRoles get 403;
// an empty list means any authenticated role is allowed.
func (m *Middleware) Handler(requiredRoles []users.Role) gin.HandlerFunc {
	if len(requiredRoles) == 0 {
		requiredRoles = users.AllRoles()
	}

	required := make(map[users.Role]struct{}, len(requiredRoles))
	for _, r := range requiredRoles {
		required[r] = struct{}{}
	}

	return func(c *gin.Context) {
		c.Set(requiredRolesKey, required)

		m.middleware.MiddlewareFunc()(c)
	}
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) > len(prefix) && strings.EqualFold(header[:len(prefix)], prefix) {
		return header[len(prefix):]
	}

	return ""
}
