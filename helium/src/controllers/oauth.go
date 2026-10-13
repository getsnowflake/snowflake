package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"

	"github.com/getsnowflake/snowflake/helium/pkg/exceptions"
	"github.com/getsnowflake/snowflake/helium/pkg/jwt"
	"github.com/getsnowflake/snowflake/helium/pkg/logger"
	"github.com/getsnowflake/snowflake/helium/pkg/middleware"
	"github.com/getsnowflake/snowflake/helium/src/service"

	"github.com/gofiber/fiber/v2"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

type OAuthController interface {
	Redirect(c *fiber.Ctx) error
	Callback(c *fiber.Ctx) error
}

type oauthController struct {
	db       *gorm.DB
	jwt      *jwt.JWT
	oauth    *oauth2.Config
	secure   bool
	services service.OAuthService
	log      *logger.Logger
}

func NewOAuthController(db *gorm.DB, j *jwt.JWT, conf *viper.Viper, svc service.OAuthService, log *logger.Logger) OAuthController {
	return &oauthController{
		db:  db,
		jwt: j,
		oauth: &oauth2.Config{
			ClientID:     conf.GetString("google.client_id"),
			ClientSecret: conf.GetString("google.client_secret"),
			RedirectURL:  conf.GetString("google.redirect_url"),
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
		secure:   conf.GetBool("http.secure"),
		services: svc,
		log:      log,
	}
}

// Redirect godoc
//
//	@Summary		Start Google sign-in
//	@Description	Initiate Google OAuth login
//	@Tags			Auth
//	@Produce		json
//	@Success		307	"Redirects to Google OAuth consent screen"
//	@Failure		500	{object}	exceptions.InternalServerError	"Failed to generate state"
//	@Router			/auth/oauth [get]
//	@ID				oauthRedirect
func (s *oauthController) Redirect(c *fiber.Ctx) error {
	state, err := generateState()
	if err != nil {
		return exceptions.InternalServer(c, "failed to generate state")
	}

	c.Cookie(&fiber.Cookie{
		Name:     "oauth_state",
		Value:    state,
		HTTPOnly: true,
		SameSite: "lax",
		Secure:   s.secure,
		MaxAge:   600,
	})

	url := s.oauth.AuthCodeURL(state, oauth2.AccessTypeOnline)

	return c.Redirect(url, fiber.StatusTemporaryRedirect)
}

// Callback godoc
//
//	@Summary		Complete Google sign-in
//	@Description	Google OAuth callback — exchanges code for token, creates/links account, returns access and refresh tokens
//	@Description	Emits: `user.created` event upon new user registration.
//	@Tags			Auth
//	@Produce		json
//	@Param			code	query		string							true	"Authorization code from Google"
//	@Param			state	query		string							true	"CSRF state token"
//	@Success		200		{object}	dto.OAuthResponse				"OAuthResponse"
//	@Failure		400		{object}	exceptions.BadRequestError		"Missing code or state"
//	@Failure		401		{object}	exceptions.UnauthorizedError	"Invalid state"
//	@Failure		500		{object}	exceptions.InternalServerError	"Failed to exchange code OR fetch user info OR generate token"
//	@Router			/auth/oauth/callback [get]
//	@ID				oauthCallback
func (s *oauthController) Callback(c *fiber.Ctx) error {
	state := c.Query("state")
	code := c.Query("code")

	if code == "" || state == "" {
		return exceptions.BadRequest(c, "missing code or state")
	}

	cookieState := c.Cookies("oauth_state")
	if cookieState == "" || cookieState != state {
		return exceptions.Unauthorized(c)
	}

	c.Cookie(&fiber.Cookie{
		Name:     "oauth_state",
		Value:    "",
		HTTPOnly: true,
		SameSite: "lax",
		Secure:   s.secure,
		MaxAge:   -1,
	})

	token, err := s.oauth.Exchange(c.Context(), code)
	if err != nil {
		return exceptions.InternalServer(c, "failed to exchange code")
	}

	client := s.oauth.Client(c.Context(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return exceptions.InternalServer(c, "failed to get user info")
	}
	defer resp.Body.Close()

	var googleUser struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		return exceptions.InternalServer(c, "failed to decode user info")
	}

	userID, err := s.services.UpsertOAuthUser(s.db, googleUser.Sub, googleUser.Email, googleUser.Name, s.log.WithContext(c).Logger, middleware.GetCorrelationID(c))
	if err != nil {
		return exceptions.InternalServer(c, "failed to complete oauth sign-in")
	}

	oauthResp, err := s.services.GenerateAuthResponse(s.db, userID)
	if err != nil {
		return exceptions.InternalServer(c, "failed to complete oauth sign-in")
	}

	return c.Status(fiber.StatusOK).JSON(oauthResp)
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
