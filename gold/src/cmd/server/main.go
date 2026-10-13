package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getsnowflake/snowflake/gold/pkg/config"
	"github.com/getsnowflake/snowflake/gold/pkg/events"
	jwtpkg "github.com/getsnowflake/snowflake/gold/pkg/jwt"
	logpkg "github.com/getsnowflake/snowflake/gold/pkg/logger"
	"github.com/getsnowflake/snowflake/gold/pkg/messaging"
	"github.com/getsnowflake/snowflake/gold/pkg/metrics"
	"github.com/getsnowflake/snowflake/gold/pkg/middleware"
	"github.com/getsnowflake/snowflake/gold/pkg/validator"
	"github.com/getsnowflake/snowflake/gold/src/db"
	"github.com/getsnowflake/snowflake/gold/src/models"
	"github.com/getsnowflake/snowflake/gold/src/outbox"
	"github.com/getsnowflake/snowflake/gold/src/reconciliation"
	"github.com/getsnowflake/snowflake/gold/src/repository"
	"github.com/getsnowflake/snowflake/gold/src/routes"
	"github.com/getsnowflake/snowflake/gold/src/service"

	"github.com/google/uuid"

	amqp091 "github.com/rabbitmq/amqp091-go"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

//	@title			Snowflake API Reference
//	@version		1.0.0
//	@description	The Snowflake API is organized around REST. This API has predictable resource-oriented URLs, accepts JSON-encoded request bodies, returns JSON-encoded responses, and uses standard HTTP response codes, authentication, and verbs.

//	@contact.name	GitHub
//	@contact.url	https://github.com/getsnowflake/snowflake/issues

//	@license.name	GNU General Public License v3.0
//	@license.url	https://www.gnu.org/licenses/gpl-3.0

//	@host		snowflake.snowye.dev
//	@BasePath	/

//	@securityDefinitions.apikey	Bearer
//	@in							header
//	@name						Authorization
//	@description				JWT access token, prefixed with Bearer.

//	@Schemes	https
func main() {
	conf := config.GetConfig()
	logger := logpkg.NewLog(conf)

	// Start RabbitMQ and defer its closure
	rmq := startRabbitMQ(conf, logger)
	defer rmq.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel to handle shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	repos := repository.NewFactory()

	// Start outbox worker (polls every 5s, publishes pending events)
	outboxWorker := outbox.NewWorker(db.GetDB(), rmq, repos, logger)
	go outboxWorker.Start(ctx)

	// Start reconciliation job (hourly)
	reconcileJob := reconciliation.NewJob(db.GetDB(), logger)
	reconcileJob.Start(time.Hour)

	// Start servers
	go startHTTPServer(conf, logger, rmq, reconcileJob)
	go startAccountCreationConsumer(rmq, logger)

	<-quit // Wait for shutdown signal

	log.Println("Shutting down the servers...")
	log.Println("All servers stopped gracefully")
}

func startRabbitMQ(conf *viper.Viper, log *logpkg.Logger) *messaging.MessagingService {
	rmq, err := messaging.NewRabbitMQ(conf.GetString("messaging.connectionString"), log)
	if err != nil {
		log.Fatal("Failed to connect to RabbitMQ", zap.Error(err))
	}

	log.Info("Connected to RabbitMQ")

	return rmq
}

func startHTTPServer(conf *viper.Viper, logger *logpkg.Logger, rmq *messaging.MessagingService, reconcileJob *reconciliation.Job) {
	validator.InitValidator(logger)

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": err.Error(),
			})
		},
	})

	// app.Use(swagger.New(
	// 	swagger.Config{
	// 		BasePath: "/",
	// 		FilePath: "./swagger.json",
	// 		Path:     "swagger",
	// 		Title:    "Swagger API Docs",
	// 	},
	// ))

	// Prometheus metrics: instrument all routes and expose at /metrics.
	// Served on the HTTP port (not proxied through nginx).
	metrics.Register(app, "gold")

	// CORS middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// Correlation ID middleware (must run before all routes)
	app.Use(middleware.CorrelationMiddleware(logger))

	// JWT Middleware
	jwt := middleware.JWTMiddleware(conf, logger)

	// Rate Limit Middleware
	rateLimit := middleware.UserRateLimitMiddleware(conf)

	repos := repository.NewFactory()
	services := service.NewServiceFactory(repos, logger)

	routes.BindHealthRoutes(app, db.GetDB(), rmq)
	routes.BindFlakeRoutes(app, db.GetDB(), jwtpkg.NewJwt(conf), jwt, services)
	routes.BindBalanceRoutes(app, db.GetDB(), jwtpkg.NewJwt(conf), jwt, services)
	routes.BindTransactionRoutes(app, db.GetDB(), jwtpkg.NewJwt(conf), jwt, rateLimit, services)
	routes.BindAdminRoutes(app, jwt, rateLimit, logger, reconcileJob)

	port := conf.GetInt("http.port")
	formattedPort := fmt.Sprintf(":%d", port)

	log.Printf("HTTP server is running on port %d\n", port)
	if err := app.Listen(formattedPort); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

func startAccountCreationConsumer(rmq *messaging.MessagingService, log *logpkg.Logger) {
	exchangeName := events.ExchangeUserCreated
	queueName := events.QueueUserCreatedAccount

	const (
		initialBackoff = time.Second
		maxBackoff     = 30 * time.Second
		maxRetries     = 5
	)
	backoff := initialBackoff

	messages, amqpCh, err := rmq.ConsumeFromExchange(exchangeName, queueName)
	if err != nil {
		for attempt := 1; attempt <= maxRetries; attempt++ {
			log.Error("Failed to start consumer, retrying",
				zap.Error(err),
				zap.String("queueName", queueName),
				zap.Duration("backoff", backoff),
				zap.Int("attempt", attempt),
			)
			time.Sleep(backoff)
			backoff = nextBackoff(backoff, maxBackoff)

			messages, amqpCh, err = rmq.ConsumeFromExchange(exchangeName, queueName)
			if err == nil {
				break
			}
		}
		if err != nil {
			log.Fatal("Failed to start consumer after retries",
				zap.Error(err),
				zap.String("queueName", queueName),
				zap.Int("maxRetries", maxRetries),
			)
		}
	}

	log.Info("Consumer started", zap.String("queueName", queueName))

	for attempt := 0; attempt < maxRetries; attempt++ {
		for msg := range messages {
			correlationID := extractCorrelationID(msg.Headers)
			msgLogger := &logpkg.Logger{Logger: log.Logger.With(zap.String("correlation_id", correlationID))}

			msgLogger.Info("Received message",
				zap.String("queueName", queueName),
				zap.String("body", string(msg.Body)),
				zap.String("contentType", msg.ContentType),
			)

			var event UserCreatedEvent
			if err := json.Unmarshal(msg.Body, &event); err != nil {
				msgLogger.Error("Failed to unmarshal message", zap.Error(err), zap.String("body", string(msg.Body)))
				msg.Nack(false, false)
				continue
			}

			if err := processUserCreated(event, msgLogger); err != nil {
				msgLogger.Error("Failed to process user.created event", zap.Error(err), zap.String("userID", event.ID))
				msg.Nack(false, true)
				continue
			}

			msg.Ack(false)
		}

		if attempt == maxRetries-1 {
			log.Error("Consumer connection lost, max retries exhausted",
				zap.String("queueName", queueName),
			)
			return
		}

		amqpCh.Close()

		log.Warn("Consumer connection lost, reconnecting",
			zap.String("queueName", queueName),
			zap.Duration("backoff", backoff),
			zap.Int("attempt", attempt+1),
		)
		time.Sleep(backoff)
		backoff = nextBackoff(backoff, maxBackoff)

		messages, amqpCh, err = rmq.ConsumeFromExchange(exchangeName, queueName)
		if err != nil {
			log.Error("Consumer connection lost, retries exhausted",
				zap.Error(err),
				zap.String("queueName", queueName),
			)
			return
		}

		log.Info("Consumer reconnected", zap.String("queueName", queueName))
	}
}

type UserCreatedEvent struct {
	ID           string  `json:"id"`
	Username     string  `json:"username"`
	Email        string  `json:"email"`
	AnnualIncome float64 `json:"annual_income"`
	Debt         float64 `json:"debt"`
	AssetsValue  float64 `json:"assets_value"`
	CreatedAt    string  `json:"created_at"`
}

func extractCorrelationID(headers amqp091.Table) string {
	if headers == nil {
		return uuid.New().String()
	}
	val, ok := headers["x-correlation-id"]
	if !ok {
		return uuid.New().String()
	}
	s, ok := val.(string)
	if !ok || s == "" {
		return uuid.New().String()
	}
	if _, err := uuid.Parse(s); err != nil {
		return uuid.New().String()
	}
	return s
}

func processUserCreated(event UserCreatedEvent, log *logpkg.Logger) error {
	log.Info("Processing user.created event",
		zap.String("ID", event.ID),
		zap.String("Username", event.Username),
		zap.String("Email", event.Email),
	)

	userID, err := uuid.Parse(event.ID)
	if err != nil {
		return fmt.Errorf("invalid user ID %q: %w", event.ID, err)
	}

	database := db.GetDB()

	var existing models.Account
	if err := database.Where("user_id = ?", userID).First(&existing).Error; err == nil {
		log.Info("Account already exists for user, skipping", zap.String("userID", event.ID))
		return nil
	}

	account := models.Account{
		UserID:               userID,
		Balance:              0,
		Status:               models.AccountStatusActive,
		ReconciliationStatus: models.AccountReconciliationStatusOK,
	}

	if err := database.Create(&account).Error; err != nil {
		return fmt.Errorf("failed to create account for user %q: %w", event.ID, err)
	}

	log.Info("Account created for user",
		zap.String("userID", event.ID),
		zap.String("accountID", account.ID.String()),
	)

	return nil
}

func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}
