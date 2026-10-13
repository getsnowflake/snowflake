package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/getsnowflake/snowflake/gold/pkg/exceptions"
	"github.com/getsnowflake/snowflake/gold/pkg/jwt"
	"github.com/getsnowflake/snowflake/gold/pkg/middleware"
	"github.com/getsnowflake/snowflake/gold/src/dto"
	"github.com/getsnowflake/snowflake/gold/src/repository"
	"github.com/getsnowflake/snowflake/gold/src/service"
	"github.com/getsnowflake/snowflake/gold/src/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	defaultTransactionLimit = 20
	maxTransactionLimit     = 100
	dateLayout              = "2006-01-02"
)

type TransactionsController interface {
	GetTransactions(ctx *fiber.Ctx) error
	GetTransactionByID(ctx *fiber.Ctx) error
	CreateTransaction(ctx *fiber.Ctx) error
	Deposit(ctx *fiber.Ctx) error
}

type transactionsController struct {
	db      *gorm.DB
	jwt     *jwt.JWT
	service service.TransactionService
}

func NewTransactionsController(db *gorm.DB, jwt *jwt.JWT, svc service.TransactionService) TransactionsController {
	return &transactionsController{db: db, jwt: jwt, service: svc}
}

// GetTransactions godoc
//
//	@Summary		List account transactions
//	@Description	Get user account transactions with optional filters and pagination
//	@Tags			Transactions
//	@Accept			json
//	@Produce		json
//	@Param			page		query		int									false	"Page number (default: 1)"
//	@Param			limit		query		int									false	"Items per page, max 100 (default: 20)"
//	@Param			direction	query		string								false	"Filter by direction relative to the account: credit (money in) or debit (money out)"	Enums(credit, debit)
//	@Param			start_date	query		string								false	"Only transactions on or after this UTC date (YYYY-MM-DD)"
//	@Param			end_date	query		string								false	"Only transactions on or before this UTC date, inclusive (YYYY-MM-DD)"
//	@Success		200			{object}	dto.PaginatedTransactionsResponse	"PaginatedTransactionsResponse"
//	@Failure		400			{object}	exceptions.BadRequestError			"Invalid direction or date filter"
//	@Failure		404			{object}	exceptions.NotFoundError			"Account not found"
//	@Failure		500			{object}	exceptions.InternalServerError		"Failed to fetch transactions"
//	@Security		Bearer
//	@Router			/account/transactions [get]
//	@ID				getTransactions
func (s *transactionsController) GetTransactions(ctx *fiber.Ctx) error {
	claims := ctx.Locals("claims").(*jwt.Claims)

	page := 1
	limit := defaultTransactionLimit
	if p := ctx.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := ctx.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			if v > maxTransactionLimit {
				v = maxTransactionLimit
			}
			limit = v
		}
	}

	direction := ctx.Query("direction")
	if direction != "" && direction != "credit" && direction != "debit" {
		return exceptions.BadRequest(ctx, "direction must be 'credit' or 'debit'")
	}

	var startDate, endDate *time.Time
	if s := ctx.Query("start_date"); s != "" {
		d, err := time.Parse(dateLayout, s)
		if err != nil {
			return exceptions.BadRequest(ctx, "start_date must be in YYYY-MM-DD format")
		}
		startDate = &d
	}
	if e := ctx.Query("end_date"); e != "" {
		d, err := time.Parse(dateLayout, e)
		if err != nil {
			return exceptions.BadRequest(ctx, "end_date must be in YYYY-MM-DD format")
		}
		if startDate != nil && startDate.After(d) {
			return exceptions.BadRequest(ctx, "start_date must not be after end_date")
		}
		// Exclusive upper bound: include the whole end_date day.
		exclusive := d.AddDate(0, 0, 1)
		endDate = &exclusive
	}

	filter := repository.TransactionFilter{
		Direction: direction,
		StartDate: startDate,
		EndDate:   endDate,
		Page:      page,
		Limit:     limit,
	}

	resp, err := s.service.GetTransactions(s.db, claims.Subject, filter)
	if err != nil {
		if errors.Is(err, service.ErrAccountNotFound) {
			return exceptions.NotFound(ctx, "Account not found")
		}
		return exceptions.InternalServer(ctx, "Failed to fetch transactions")
	}

	return ctx.Status(http.StatusOK).JSON(resp)
}

// GetTransactionByID godoc
//
//	@Summary		Get a transaction
//	@Description	Get a specific transaction by ID (user must be sender or receiver)
//	@Tags			Transactions
//	@Accept			json
//	@Produce		json
//	@Param			id	path		string							true	"Transaction ID"
//	@Success		200	{object}	dto.TransactionResponse			"TransactionResponse"
//	@Failure		404	{object}	exceptions.NotFoundError		"Account or transaction not found"
//	@Failure		500	{object}	exceptions.InternalServerError	"Failed to fetch transaction"
//	@Security		Bearer
//	@Router			/account/transactions/{id} [get]
//	@ID				getTransactionByID
func (s *transactionsController) GetTransactionByID(ctx *fiber.Ctx) error {
	claims := ctx.Locals("claims").(*jwt.Claims)
	id, err := uuid.Parse(ctx.Params("id"))
	if err != nil {
		return exceptions.BadRequest(ctx, "Invalid transaction ID")
	}

	resp, err := s.service.GetTransactionByID(s.db, claims.Subject, id)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrAccountNotFound):
			return exceptions.NotFound(ctx, "Account not found")
		case errors.Is(err, service.ErrTransactionNotFound):
			return exceptions.NotFound(ctx, "Transaction not found")
		default:
			return exceptions.InternalServer(ctx, "Failed to fetch transaction")
		}
	}

	return ctx.Status(http.StatusOK).JSON(resp)
}

// CreateTransaction godoc
//
//	@Summary		Transfer funds
//	@Description	Create a new transaction/transfer between accounts
//	@Description	Emits: `transaction.received` event (outbox pattern, at-least-once) upon successful transfer completion.
//	@Tags			Transactions
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.CreateTransferRequest		true	"Create Transfer Request"
//	@Success		200		{object}	dto.TransactionResponse			"Previously completed transaction returned for an idempotent retry"
//	@Success		201		{object}	dto.TransactionResponse			"TransactionResponse"
//	@Failure		400		{object}	exceptions.BadRequestError		"Invalid amount, account status, or limits exceeded"
//	@Failure		404		{object}	exceptions.NotFoundError		"Account or receiver flake key not found"
//	@Failure		500		{object}	exceptions.InternalServerError	"Transfer failed"
//	@Security		Bearer
//	@Router			/account/transactions/transfer [post]
//	@ID				createTransaction
func (s *transactionsController) CreateTransaction(ctx *fiber.Ctx) error {
	claims := ctx.Locals("claims").(*jwt.Claims)

	body := new(dto.CreateTransferRequest)
	if err := utils.ParseRequest(ctx, body); err != nil {
		return err
	}

	resp, err := s.service.CreateTransaction(s.db, claims.Subject, *body, middleware.GetCorrelationID(ctx))
	if err != nil {
		var idempotent *service.IdempotentTransactionError
		if errors.As(err, &idempotent) {
			return ctx.Status(http.StatusOK).JSON(idempotent.Response)
		}
		switch {
		case errors.Is(err, service.ErrAmountTooLow),
			errors.Is(err, service.ErrAmountTooHigh),
			errors.Is(err, service.ErrAccountNotActive),
			errors.Is(err, service.ErrAccountReconciliation),
			errors.Is(err, service.ErrSelfTransfer),
			errors.Is(err, service.ErrInsufficientFunds),
			errors.Is(err, service.ErrDailyLimitExceeded),
			errors.Is(err, service.ErrDailyCountExceeded):
			return exceptions.BadRequest(ctx, err.Error())
		case errors.Is(err, service.ErrAccountNotFound),
			errors.Is(err, service.ErrFlakeNotFound):
			return exceptions.NotFound(ctx, err.Error())
		default:
			return exceptions.InternalServer(ctx, "Transfer failed")
		}
	}

	return ctx.Status(http.StatusCreated).JSON(resp)
}

// Deposit godoc
//
//	@Summary		Deposit funds
//	@Description	Deposit funds into an account
//	@Description	Emits: `transaction.received` event (outbox pattern, at-least-once) upon successful deposit completion.
//	@Tags			Transactions
//	@Accept			json
//	@Produce		json
//	@Param			body	body		dto.DepositRequest				true	"Deposit Request"
//	@Success		200		{object}	dto.TransactionResponse			"Previously completed transaction returned for an idempotent retry"
//	@Success		201		{object}	dto.TransactionResponse			"TransactionResponse"
//	@Failure		400		{object}	exceptions.BadRequestError		"Invalid amount or account status"
//	@Failure		403		{object}	exceptions.ForbiddenError		"Account does not belong to authenticated user"
//	@Failure		404		{object}	exceptions.NotFoundError		"Account not found"
//	@Failure		500		{object}	exceptions.InternalServerError	"Deposit failed"
//	@Security		Bearer
//	@Router			/account/transactions/deposit [post]
//	@ID				deposit
func (s *transactionsController) Deposit(ctx *fiber.Ctx) error {
	claims := ctx.Locals("claims").(*jwt.Claims)

	body := new(dto.DepositRequest)
	if err := utils.ParseRequest(ctx, body); err != nil {
		return err
	}

	resp, err := s.service.Deposit(s.db, claims.Subject, *body, middleware.GetCorrelationID(ctx))
	if err != nil {
		var idempotent *service.IdempotentTransactionError
		if errors.As(err, &idempotent) {
			return ctx.Status(http.StatusOK).JSON(idempotent.Response)
		}
		switch {
		case errors.Is(err, service.ErrAmountTooLow),
			errors.Is(err, service.ErrAmountTooHigh),
			errors.Is(err, service.ErrAccountNotActive),
			errors.Is(err, service.ErrInvalidTokenSubject):
			return exceptions.BadRequest(ctx, err.Error())
		case errors.Is(err, service.ErrForbidden):
			return exceptions.Forbidden(ctx, err.Error())
		case errors.Is(err, service.ErrAccountNotFound):
			return exceptions.NotFound(ctx, err.Error())
		default:
			return exceptions.InternalServer(ctx, "Deposit failed")
		}
	}

	return ctx.Status(http.StatusCreated).JSON(resp)
}
