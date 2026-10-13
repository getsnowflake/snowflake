package routes

import (
	"context"
	"net/http"

	"github.com/getsnowflake/snowflake/helium/pkg/health"
	"github.com/getsnowflake/snowflake/helium/pkg/messaging"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func BindHealthRoutes(app *fiber.App, db *gorm.DB, rmq *messaging.MessagingService) {
	hs := health.NewService()
	hs.Register("database", health.NewDatabaseChecker(db))
	hs.Register("rabbitmq", health.FuncChecker(func(ctx context.Context) error {
		return rmq.HealthCheck()
	}))

	router := app.Group("/auth")
	router.Get("/health", healthHandler(hs))
}

// healthHandler godoc
//
//	@Summary		Check identity service health
//	@Description	Service health check (database + RabbitMQ)
//	@Tags			Health
//	@Produce		json
//	@Success		200	{object}	health.HealthResponse	"Service healthy"
//	@Failure		503	{object}	health.HealthResponse	"Service unhealthy"
//	@Router			/auth/health [get]
//	@ID				heliumHealth
func healthHandler(hs *health.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		status, components := hs.Check(context.Background())
		statusText := "healthy"
		if status == http.StatusServiceUnavailable {
			statusText = "unhealthy"
		}
		return c.Status(status).JSON(fiber.Map{
			"status":     statusText,
			"components": components,
		})
	}
}
