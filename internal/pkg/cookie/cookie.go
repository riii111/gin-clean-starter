package cookie

import (
	"net/http"
	"time"

	"gin-clean-starter/internal/pkg/config"

	"github.com/gin-gonic/gin"
)

const (
	AccessTokenCookieName  = "access_token"
	RefreshTokenCookieName = "refresh_token"
)

func SetTokenCookies(c *gin.Context, cfg config.CookieConfig, accessToken, refreshToken string, accessExpiry, refreshExpiry time.Duration) {
	c.SetSameSite(getSameSite(cfg.SameSite))

	c.SetCookie(
		AccessTokenCookieName,
		accessToken,
		int(accessExpiry.Seconds()),
		"/",
		cfg.Domain,
		cfg.Secure,
		true, // HttpOnly
	)

	c.SetCookie(
		RefreshTokenCookieName,
		refreshToken,
		int(refreshExpiry.Seconds()),
		"/",
		cfg.Domain,
		cfg.Secure,
		true, // HttpOnly
	)
}

func ClearTokenCookies(c *gin.Context, cfg config.CookieConfig) {
	c.SetSameSite(getSameSite(cfg.SameSite))

	c.SetCookie(
		AccessTokenCookieName,
		"",
		-1,
		"/",
		cfg.Domain,
		cfg.Secure,
		true,
	)

	c.SetCookie(
		RefreshTokenCookieName,
		"",
		-1,
		"/",
		cfg.Domain,
		cfg.Secure,
		true,
	)
}

func GetAccessToken(c *gin.Context) string {
	token, _ := c.Cookie(AccessTokenCookieName)
	return token
}

func GetRefreshToken(c *gin.Context) string {
	token, _ := c.Cookie(RefreshTokenCookieName)
	return token
}

func getSameSite(sameSite string) http.SameSite {
	switch sameSite {
	case "Strict":
		return http.SameSiteStrictMode
	case "Lax":
		return http.SameSiteLaxMode
	case "None":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
