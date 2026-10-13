package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/getsnowflake/snowflake/helium/pkg/config"
	"github.com/getsnowflake/snowflake/helium/pkg/logger"
	"github.com/getsnowflake/snowflake/helium/pkg/messaging"
	"github.com/getsnowflake/snowflake/helium/pkg/metrics"
	"github.com/getsnowflake/snowflake/helium/pkg/validator"
	"github.com/getsnowflake/snowflake/helium/src/db"
	grpcs "github.com/getsnowflake/snowflake/helium/src/grpc"
	"github.com/getsnowflake/snowflake/helium/pkg/middleware"
	"github.com/getsnowflake/snowflake/helium/src/routes"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

//	@title			Snowflake API Reference
//	@version		1.0.0
//	@description	The Snowflake API is organized around REST. This API has predictable resource-oriented URLs, accepts JSON-encoded request bodies, returns JSON-encoded responses, and uses standard HTTP response codes, authentication, and verbs.

//	@contact.name	GitHub
//	@contact.url	https://github.com/getsnowflake/snowflake/helium/issues

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
	logger := logger.NewLog(conf)

	// Start RabbitMQ and defer its closure
	rmq := startRabbitMQ(conf, logger)
	defer rmq.Close()

	// Channel to handle shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start servers
	go startHTTPServer(conf, logger, rmq)
	go startGRPCServer(conf, logger)

	<-quit // Wait for shutdown signal

	log.Println("Shutting down the servers...")
	log.Println("All servers stopped gracefully")
}

func startRabbitMQ(conf *viper.Viper, log *logger.Logger) *messaging.MessagingService {
	rmq, err := messaging.NewRabbitMQ(conf.GetString("messaging.connectionString"), log)
	if err != nil {
		log.Fatal("Failed to connect to RabbitMQ", zap.Error(err))
	}

	log.Info("Connected to RabbitMQ")

	return rmq
}

func startHTTPServer(conf *viper.Viper, logger *logger.Logger, rmq *messaging.MessagingService) {
	validator.InitValidator(logger)

	app := fiber.New(fiber.Config{
		ProxyHeader:    "X-Forwarded-For",
		TrustedProxies: []string{"0.0.0.0/0"},
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
	metrics.Register(app, "helium")

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
	rateLimit := middleware.IPRateLimitMiddleware(conf)

	// Bind routes
	routes.BindHealthRoutes(app, db.GetDB(), rmq)
	routes.BindAuthRoutes(app, jwt, rateLimit, logger, rmq)

	port := conf.GetInt("http.port")
	formattedPort := fmt.Sprintf(":%d", port)

	log.Printf("HTTP server is running on port %d\n", port)
	if err := app.Listen(formattedPort); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

func startGRPCServer(conf *viper.Viper, l *logger.Logger) {
	grpcPort := conf.GetInt("grpc.port")
	grpcFormattedPort := fmt.Sprintf(":%d", grpcPort)

	lis, err := net.Listen("tcp", grpcFormattedPort)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpcs.NewServer(l.Logger)
	log.Printf("gRPC server is running on port %d\n", grpcPort)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}
