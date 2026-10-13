package controllers

import (
	"github.com/getsnowflake/snowflake/helium/pkg/exceptions"
	"github.com/getsnowflake/snowflake/helium/pkg/jwt"
	"github.com/getsnowflake/snowflake/helium/pkg/logger"
	"github.com/getsnowflake/snowflake/helium/pkg/middleware"
	"github.com/getsnowflake/snowflake/helium/src/service"
	"github.com/getsnowflake/snowflake/helium/src/dto"
	"github.com/getsnowflake/snowflake/helium/src/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type AuthController interface {
	Profile(ctx *fiber.Ctx) error
	Register(ctx *fiber.Ctx) error
	Login(ctx *fiber.Ctx) error
	Refresh(ctx *fiber.Ctx) error
	Logout(ctx *fiber.Ctx) error
}

type authController struct {
	db       *gorm.DB
	jwt      *jwt.JWT
	services service.AuthService
	log      *logger.Logger
}

func NewAuthController(db *gorm.DB, j *jwt.JWT, svc service.AuthService, log *logger.Logger) AuthController {
	return &authController{db: db, jwt: j, services: svc, log: log}
}

// Profile godoc
//
//	@Summary		Get user profile
//	@Description	Return the authenticated user's identity and financial profile.
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	dto.ProfileResponse				"ProfileResponse"
//	@Failure		401	{object}	exceptions.UnauthorizedError	"Unauthorized"
//	@Failure		500	{object}	exceptions.InternalServerError	"Failed to fetch user"
//	@Security		Bearer
//	@Router			/auth/profile [get]
//	@ID				profile
func (s *authController) Profile(c *fiber.Ctx) error {
	claims := c.Locals("claims").(*jwt.Claims)

	resp, err := s.services.Profile(s.db, claims.Subject)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			return exceptions.Unauthorized(c)
		}
		return exceptions.InternalServer(c, "failed to fetch user")
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

// Register godoc
//
//	@Summary		Register a user
//	@Description	Register a new user.
//	@Description	Emits: `user.created` event upon successful registration.
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.RegisterRequest				true	"Register Request"
//	@Success		200		{object}	dto.RegisterResponse			"RegisterResponse"
//	@Failure		400		{object}	exceptions.BadRequestError		"Invalid request body"
//	@Failure		409		{object}	exceptions.ConflictError		"Email already taken"
//	@Failure		500		{object}	exceptions.InternalServerError	"Failed to hash password or generate token"
//	@Router			/auth/register [post]
//	@ID				register
func (s *authController) Register(c *fiber.Ctx) error {
	body := new(dto.RegisterRequest)
	if err := utils.ParseRequest(c, body); err != nil {
		return err
	}

	resp, err := s.services.Register(s.db, *body, s.log.WithContext(c).Logger, middleware.GetCorrelationID(c))
	if err != nil {
		if errors.Is(err, service.ErrEmailAlreadyTaken) {
			return exceptions.Conflict(c, "Email already taken")
		}
		return exceptions.InternalServer(c, "failed to register user")
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

// Login godoc
//
//	@Summary		Log in
//	@Description	Login
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.LoginRequest	true	"Login Request"
//	@Success		200		{object}	dto.LoginResponse
//	@Failure		400		{object}	exceptions.BadRequestError		"Invalid request body"
//	@Failure		401		{object}	exceptions.UnauthorizedError	"Wrong email or password"
//	@Failure		500		{object}	exceptions.InternalServerError	"Failed to sign in"
//	@Router			/auth/login [post]
//	@ID				login
func (s *authController) Login(c *fiber.Ctx) error {
	body := new(dto.LoginRequest)
	if err := utils.ParseRequest(c, body); err != nil {
		return err
	}

	resp, err := s.services.Login(s.db, *body)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return exceptions.Unauthorized(c)
		}
		return exceptions.InternalServer(c, "failed to sign in")
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

// Refresh godoc
//
//	@Summary		Refresh access tokens
//	@Description	Exchange a refresh token for a new access token and refresh token (token rotation).
//	@Description	The provided refresh token is revoked and a new pair is issued.
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.RefreshRequest				true	"Refresh Request"
//	@Success		200		{object}	dto.RefreshResponse				"Refreshed tokens"
//	@Failure		400		{object}	exceptions.BadRequestError		"Invalid request body"
//	@Failure		401		{object}	exceptions.UnauthorizedError	"Invalid, revoked, or expired refresh token"
//	@Failure		500		{object}	exceptions.InternalServerError	"Failed to refresh tokens"
//	@Router			/auth/refresh [post]
//	@ID				refresh
func (s *authController) Refresh(c *fiber.Ctx) error {
	body := new(dto.RefreshRequest)
	if err := utils.ParseRequest(c, body); err != nil {
		return err
	}

	resp, err := s.services.Refresh(s.db, body.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrRefreshTokenNotFound) || errors.Is(err, service.ErrInvalidCredentials) {
			return exceptions.Unauthorized(c)
		}
		return exceptions.InternalServer(c, "failed to refresh tokens")
	}

	return c.Status(fiber.StatusOK).JSON(resp)
}

// Logout godoc
//
//	@Summary		Log out
//	@Description	Revoke a refresh token, invalidating the session.
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			body	body	dto.LogoutRequest	true	"Logout Request"
//	@Success		204		"No Content"
//	@Failure		400		{object}	exceptions.BadRequestError		"Invalid request body"
//	@Failure		401		{object}	exceptions.UnauthorizedError	"Invalid or already revoked refresh token"
//	@Router			/auth/logout [post]
//	@ID				logout
func (s *authController) Logout(c *fiber.Ctx) error {
	body := new(dto.LogoutRequest)
	if err := utils.ParseRequest(c, body); err != nil {
		return err
	}

	if err := s.services.Logout(s.db, body.RefreshToken); err != nil {
		if errors.Is(err, service.ErrRefreshTokenNotFound) {
			return exceptions.Unauthorized(c)
		}
		return exceptions.InternalServer(c, "failed to sign out")
	}

	return c.SendStatus(fiber.StatusNoContent)
}
