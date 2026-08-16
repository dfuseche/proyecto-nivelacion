package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"okf-converter/internal/config"
	"okf-converter/internal/domain"
	"okf-converter/internal/repository"
	"okf-converter/internal/service"
)

func main() {
	cfg := config.LoadConfig()

	// 1. Inicializar Base de Datos PostgreSQL
	db, err := repository.NewDB(cfg.GetDSN())
	if err != nil {
		log.Fatalf("[FATAL] Fallo al conectar con la base de datos: %v", err)
	}

	// 2. Inicializar Almacenamiento de Objetos MinIO
	storage, err := repository.NewStorage(cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, cfg.MinIOUseSSL)
	if err != nil {
		log.Fatalf("[FATAL] Fallo al conectar con MinIO: %v", err)
	}

	// 3. Inicializar Cola de Mensajes RabbitMQ
	queue, err := repository.NewQueue(cfg.RabbitMQURL, cfg.RabbitMQQueue)
	if err != nil {
		log.Fatalf("[FATAL] Fallo al conectar con RabbitMQ: %v", err)
	}
	defer queue.Close()

	// 4. Inicializar Servicios
	authService := service.NewAuthService(db, cfg.JWTSecret)
	jobService := service.NewJobService(db, storage, queue)

	// 5. Configurar Servidor Gin HTTP
	r := gin.Default()

	// Habilitar CORS para el Frontend
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	r.Use(cors.New(corsConfig))

	// Endpoint de Salud (Healthcheck)
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "UP", "message": "API de Conversión OKF operativa"})
	})

	// Rutas Públicas de Autenticación
	apiGroup := r.Group("/api")
	authGroup := apiGroup.Group("/auth")
	{
		authGroup.POST("/register", func(c *gin.Context) {
			var req domain.RegisterRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Datos de registro inválidos: " + err.Error()})
				return
			}

			resp, err := authService.Register(req)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusCreated, resp)
		})

		authGroup.POST("/login", func(c *gin.Context) {
			var req domain.LoginRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Datos de inicio de sesión inválidos"})
				return
			}

			resp, err := authService.Login(req)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, resp)
		})
	}

	// Middleware de Autenticación JWT para Rutas Protegidas
	protectedGroup := apiGroup.Group("/jobs")
	protectedGroup.Use(authMiddleware(authService))
	{
		// Carga de Documentos (Retorno Inmediato en ms)
		protectedGroup.POST("/upload", func(c *gin.Context) {
			userIDVal, _ := c.Get("user_id")
			userID := userIDVal.(uuid.UUID)

			fileHeader, err := c.FormFile("file")
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Archivo no proporcionado en la solicitud"})
				return
			}

			file, err := fileHeader.Open()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error al abrir el archivo subido"})
				return
			}
			defer file.Close()

			job, err := jobService.CreateJob(c.Request.Context(), userID, fileHeader.Filename, file, fileHeader.Size)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Fallo al crear el trabajo: " + err.Error()})
				return
			}

			c.JSON(http.StatusAccepted, gin.H{
				"message": "Archivo recibido correctamente. El procesamiento ha iniciado en segundo plano.",
				"job_id":  job.ID,
				"status":  job.Status,
			})
		})

		// Listar Trabajos del Usuario
		protectedGroup.GET("", func(c *gin.Context) {
			userIDVal, _ := c.Get("user_id")
			userID := userIDVal.(uuid.UUID)

			jobs, err := jobService.GetJobsByUserID(userID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, jobs)
		})

		// Consultar Estado y Trazabilidad del Trabajo (Con Aislamiento)
		protectedGroup.GET("/:id", func(c *gin.Context) {
			userIDVal, _ := c.Get("user_id")
			userID := userIDVal.(uuid.UUID)

			jobIDStr := c.Param("id")
			jobID, err := uuid.Parse(jobIDStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "ID de trabajo inválido"})
				return
			}

			job, logs, err := jobService.GetJobByID(jobID, userID)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"job":  job,
				"logs": logs,
			})
		})

		// Eliminar Trabajo (Con Aislamiento)
		protectedGroup.DELETE("/:id", func(c *gin.Context) {
			userIDVal, _ := c.Get("user_id")
			userID := userIDVal.(uuid.UUID)

			jobIDStr := c.Param("id")
			jobID, err := uuid.Parse(jobIDStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "ID de trabajo inválido"})
				return
			}

			if err := jobService.DeleteJob(c.Request.Context(), jobID, userID); err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "Trabajo eliminado exitosamente"})
		})

		// Descargar Bundle OKF (Con Aislamiento)
		protectedGroup.GET("/:id/download", func(c *gin.Context) {
			userIDVal, _ := c.Get("user_id")
			userID := userIDVal.(uuid.UUID)

			jobIDStr := c.Param("id")
			jobID, err := uuid.Parse(jobIDStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "ID de trabajo inválido"})
				return
			}

			stream, downloadFilename, err := jobService.GetDownloadStream(context.Background(), jobID, userID)
			if err != nil {
				c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
			defer stream.Close()

			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", downloadFilename))
			c.Header("Content-Type", "application/zip")
			c.DataFromReader(http.StatusOK, -1, "application/zip", stream, nil)
		})
	}

	log.Printf("[API] Servidor HTTP escuchando en el puerto %s...", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("[FATAL] Error en el servidor API: %v", err)
	}
}

func authMiddleware(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || len(authHeader) < 8 || authHeader[:7] != "Bearer " {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Cabecera Authorization requerida (Bearer token)"})
			c.Abort()
			return
		}

		tokenString := authHeader[7:]
		claims, err := authService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token inválido o expirado: " + err.Error()})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Next()
	}
}
