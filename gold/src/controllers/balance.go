package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/getsnowflake/snowflake/gold/pkg/exceptions"
	"github.com/getsnowflake/snowflake/gold/pkg/jwt"
	"github.com/getsnowflake/snowflake/gold/src/service"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

const (
	defaultBalanceHistoryLimit = 20
	maxBalanceHistoryLimit     = 100
)

type BalanceController interface {
	GetBalance(ctx *fiber.Ctx) error
	GetBalanceHistory(ctx *fiber.Ctx) error
}

type balanceController struct {
	db      *gorm.DB
	jwt     *jwt.JWT
	service service.BalanceService
}

func NewBalanceController(db *gorm.DB, jwt *jwt.JWT, svc service.BalanceService) BalanceController {
	return &balanceController{db: db, jwt: jwt, service: svc}
}

// GetBalance godoc
//
//	@Summary		Get account balance
//	@Description	Get user account balance
//	@Tags			Balance
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	dto.BalanceResponse			"BalanceResponse"
//	@Failure		404	{object}	exceptions.NotFoundError	"Account not found"
//	@Security		Bearer
//	@Router			/account/balance [get]
//	@ID				getBalance
func (s *balanceController) GetBalance(ctx *fiber.Ctx) error {
	claims := ctx.Locals("claims").(*jwt.Claims)

	resp, err := s.service.GetBalance(s.db, claims.Subject)
	if err != nil {
		if errors.Is(err, service.ErrAccountNotFound) {
			return exceptions.NotFound(ctx, "Account not found")
		}
		return exceptions.InternalServer(ctx, "Failed to fetch balance")
	}

	return ctx.Status(http.StatusOK).JSON(resp)
}

// GetBalanceHistory godoc
//
//	@Summary		List account balance history
//	@Description	Get user account balance transaction history with pagination
//	@Tags			Balance
//	@Accept			json
//	@Produce		json
//	@Param			page	query		int								false	"Page number (default: 1)"
//	@Param			limit	query		int								false	"Items per page, max 100 (default: 20)"
//	@Success		200		{object}	dto.BalanceHistoryResponse		"BalanceHistoryResponse"
//	@Failure		404		{object}	exceptions.NotFoundError		"Account not found"
//	@Failure		500		{object}	exceptions.InternalServerError	"Failed to fetch balance history"
//	@Security		Bearer
//	@Router			/account/balance/history [get]
//	@ID				getBalanceHistory
func (s *balanceController) GetBalanceHistory(ctx *fiber.Ctx) error {
	claims := ctx.Locals("claims").(*jwt.Claims)

	page := 1
	limit := defaultBalanceHistoryLimit

	if p := ctx.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := ctx.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			if v > maxBalanceHistoryLimit {
				v = maxBalanceHistoryLimit
			}
			limit = v
		}
	}

	resp, err := s.service.GetBalanceHistory(s.db, claims.Subject, page, limit)
	if err != nil {
		if errors.Is(err, service.ErrAccountNotFound) {
			return exceptions.NotFound(ctx, "Account not found")
		}
		return exceptions.InternalServer(ctx, "Failed to fetch balance history")
	}

	return ctx.Status(http.StatusOK).JSON(resp)
}
