package auth

import (
	"net/http"
	"strings"
	"time"

	jwt "github.com/appleboy/gin-jwt/v3"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/faraquic/lotty-ab-platform/pkg/api"
	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/logger"
	"github.com/faraquic/lotty-ab-platform/services/panel/domain/users"
)

const (
	Realm = "labp-panel"

	CtxUserIDKey = logger.FieldUserID

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
		IdentityKey:   CtxUserIDKey,
		TokenLookup:   "header: Authorization: Bearer",
		TokenHeadName: "Bearer",
		TimeFunc:      time.Now,

		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			sub, _ := claims["sub"].(string)

			return sub
		},

		Authorizer: func(c *gin.Context, identity interface{}) bool {
			id, _ := identity.(string)
			token := bearerToken(c.Request.Header.Get("Authorization"))

			if id == "" || !svc.SessionActive(c.Request.Context(), token, id) {
				c.Set(sessionRevoked, true)
				return false
			}

			c.Set(CtxUserIDKey, id)

			raw, exists := c.Get(requiredRolesKey)
			if !exists {
				return true
			}

			required, _ := raw.(map[users.Role]struct{})
			role, _ := jwt.ExtractClaims(c)["role"].(string)
			_, allowed := required[users.Role(role)]

			return allowed
		},

		Unauthorized: func(c *gin.Context, code int, message string) {
			errCode := api.Unauthorized

			if code == http.StatusForbidden {
				if revoked, _ := c.Get(sessionRevoked); revoked == true {
					code = http.StatusUnauthorized
					message = "token revoked"
					logger.SetErrorType(c, logger.ErrorTypeAuthenticationFailed)
					log.Warn(
						"auth: revoked token used",
						zap.String(logger.FieldClientAddress, c.ClientIP()),
					)
				} else {
					errCode = api.Forbidden
					logger.SetErrorType(c, logger.ErrorTypeAuthorizationDenied)
					log.Warn(
						"auth: role denied",
						zap.String(logger.FieldClientAddress, c.ClientIP()),
					)
				}
			} else {
				logger.SetErrorType(c, logger.ErrorTypeAuthenticationFailed)
				log.Warn(
					"auth: authentication failed",
					zap.Int(logger.FieldResponseCode, code),
					zap.String(logger.FieldClientAddress, c.ClientIP()),
				)
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
