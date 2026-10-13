package controllers

import (
	"errors"
	"net/http"

	"github.com/getsnowflake/snowflake/gold/pkg/exceptions"
	"github.com/getsnowflake/snowflake/gold/pkg/jwt"
	"github.com/getsnowflake/snowflake/gold/src/dto"
	"github.com/getsnowflake/snowflake/gold/src/service"
	"github.com/getsnowflake/snowflake/gold/src/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FlakeController interface {
	CreateFlake(ctx *fiber.Ctx) error
	GetFlakes(ctx *fiber.Ctx) error
	DeleteFlake(ctx *fiber.Ctx) error
	PublicLookupFlake(ctx *fiber.Ctx) error
}

type flakeController struct {
	db      *gorm.DB
	jwt     *jwt.JWT
	service service.FlakeService
}

func NewFlakeController(db *gorm.DB, jwt *jwt.JWT, svc service.FlakeService) FlakeController {
	return &flakeController{db: db, jwt: jwt, service: svc}
}

// CreateFlake godoc
//
//	@Summary		Create a FLAKE key
//	@Description	Create a new flake key for the user account
//	@Tags			Flakes
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.CreateFlakeRequest			true	"Create Flake Request"
//	@Success		201		{object}	dto.FlakeResponse				"FlakeResponse"
//	@Failure		400		{object}	exceptions.BadRequestError		"Invalid flake key or maximum limit reached"
//	@Failure		404		{object}	exceptions.NotFoundError		"Account not found"
//	@Failure		500		{object}	exceptions.InternalServerError	"Failed to create flake key"
//	@Security		Bearer
//	@Router			/account/flake [post]
//	@ID				createFlake
func (s *flakeController) CreateFlake(ctx *fiber.Ctx) error {
	claims := ctx.Locals("claims").(*jwt.Claims)

	body := new(dto.CreateFlakeRequest)
	if err := utils.ParseRequest(ctx, body); err != nil {
		return err
	}

	if err := utils.ValidateFlakeKeyValue(body.KeyType, body.KeyValue); err != nil {
		return exceptions.BadRequest(ctx, err.Error())
	}

	resp, err := s.service.CreateFlake(s.db, claims.Subject, *body)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAccountNotFound):
			return exceptions.NotFound(ctx, "Account not found")
		case errors.Is(err, service.ErrAccountReconciliation),
			errors.Is(err, service.ErrDuplicateFlakeType),
			errors.Is(err, service.ErrFlakeLimitReached),
			errors.Is(err, service.ErrFlakeKeyConflict):
			return exceptions.BadRequest(ctx, err.Error())
		default:
			return exceptions.InternalServer(ctx, "Failed to create flake key")
		}
	}

	return ctx.Status(http.StatusCreated).JSON(resp)
}

// GetFlakes godoc
//
//	@Summary		List account FLAKE keys
//	@Description	Get all flake keys for the user account
//	@Tags			Flakes
//	@Accept			json
//	@Produce		json
//	@Success		200	{array}		dto.FlakeResponse				"Array of FlakeResponse"
//	@Failure		404	{object}	exceptions.NotFoundError		"Account not found"
//	@Failure		500	{object}	exceptions.InternalServerError	"Failed to fetch flake keys"
//	@Security		Bearer
//	@Router			/account/flake [get]
//	@ID				getFlakes
func (s *flakeController) GetFlakes(ctx *fiber.Ctx) error {
	claims := ctx.Locals("claims").(*jwt.Claims)

	resp, err := s.service.GetFlakes(s.db, claims.Subject)
	if err != nil {
		if errors.Is(err, service.ErrAccountNotFound) {
			return exceptions.NotFound(ctx, "Account not found")
		}
		return exceptions.InternalServer(ctx, "Failed to fetch flake keys")
	}

	return ctx.Status(http.StatusOK).JSON(resp)
}

// DeleteFlake godoc
//
//	@Summary		Deactivate a FLAKE key
//	@Description	Deactivate a flake key
//	@Tags			Flakes
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string							true	"Flake key ID"
//	@Success		200	{object}	dto.FlakeResponse				"FlakeResponse"
//	@Failure		400	{object}	exceptions.BadRequestError		"Flake key is already inactive"
//	@Failure		404	{object}	exceptions.NotFoundError		"Account or flake key not found"
//	@Failure		500	{object}	exceptions.InternalServerError	"Failed to deactivate flake key"
//	@Security		Bearer
//	@Router			/account/flake/{id} [delete]
//	@ID				deleteFlake
func (s *flakeController) DeleteFlake(ctx *fiber.Ctx) error {
	claims := ctx.Locals("claims").(*jwt.Claims)
	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return exceptions.BadRequest(ctx, "Invalid flake key ID")
	}

	resp, err := s.service.DeleteFlake(s.db, claims.Subject, id)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAccountNotFound):
			return exceptions.NotFound(ctx, "Account not found")
		case errors.Is(err, service.ErrFlakeNotFound):
			return exceptions.NotFound(ctx, "Flake key not found")
		case errors.Is(err, service.ErrFlakeAlreadyInactive):
			return exceptions.BadRequest(ctx, err.Error())
		default:
			return exceptions.InternalServer(ctx, "Failed to deactivate flake key")
		}
	}

	return ctx.Status(http.StatusOK).JSON(resp)
}

// PublicLookupFlake godoc
//
//	@Summary		Look up a FLAKE key
//	@Description	Publicly lookup account info by flake key (no authentication required)
//	@Tags			Flakes
//	@Accept			json
//	@Produce		json
//	@Param			key_value	query		string						true	"Flake key value to lookup"
//	@Success		200			{object}	dto.LookupFlakeResponse		"LookupFlakeResponse"
//	@Failure		400			{object}	exceptions.BadRequestError	"Missing key_value parameter"
//	@Failure		404			{object}	exceptions.NotFoundError	"Flake key or account not found"
//	@Router			/account/flake/lookup [get]
//	@ID				publicLookupFlake
func (s *flakeController) PublicLookupFlake(ctx *fiber.Ctx) error {
	keyValue := ctx.Query("key_value")
	if keyValue == "" {
		return exceptions.BadRequest(ctx, "key_value query parameter is required")
	}

	resp, err := s.service.PublicLookup(s.db, keyValue)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrFlakeNotFound):
			return exceptions.NotFound(ctx, "Flake key not found")
		case errors.Is(err, service.ErrAccountNotFound):
			return exceptions.NotFound(ctx, "Account not found")
		default:
			return exceptions.InternalServer(ctx, "Failed to lookup flake key")
		}
	}

	return ctx.Status(http.StatusOK).JSON(resp)
}
