package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/rueidis"
	"go.uber.org/zap"

	libauth "github.com/faraquic/lotty-ab-platform/pkg/auth"
	"github.com/faraquic/lotty-ab-platform/pkg/config"
	"github.com/faraquic/lotty-ab-platform/pkg/middleware"
	"github.com/faraquic/lotty-ab-platform/pkg/snapshot"
	authdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/auth"
	experimentsdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/experiments"
	flagsdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/flags"
	healthdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/health"
	metricsdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/metrics"
	reviewsdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/reviews"
	usersdomain "github.com/faraquic/lotty-ab-platform/services/panel/domain/users"
	panelsnapshot "github.com/faraquic/lotty-ab-platform/services/panel/snapshot"
)

func newRouter(log *zap.Logger, cfg *config.Config, pool *pgxpool.Pool, redisClient *rueidis.Client, s3Client *s3.Client) (*gin.Engine, panelsnapshot.Refresher, *snapshot.Reader) {
	r := gin.New()
	r.HandleMethodNotAllowed = true

	addMiddleware(r, cfg, log)

	apiV1 := r.Group("/api/v1/panel")

	snapWriter := snapshot.NewWriter(redisClient, log)
	snapReader := snapshot.NewReader(redisClient, log)
	healthdomain.NewHandler(pool, redisClient, s3Client, snapReader, cfg.Environment, log).RegisterRoutes(apiV1)

	tokenizer := libauth.NewJWTManager(cfg.Auth.JWT.SecretKey, cfg.Auth.JWT.TTL)
	authRepo := authdomain.NewRepository(pool)
	authSvc := authdomain.NewService(authRepo, tokenizer, redisClient, cfg.Auth.JWT.TTL, log)
	authHandler := authdomain.NewHandler(authSvc, log)
	authHandler.RegisterRoutes(apiV1)

	repo := usersdomain.NewRepository(pool)
	var storage usersdomain.Storage
	if s3Client != nil {
		storage = usersdomain.NewS3Storage(s3Client, cfg.Database.S3.Bucket, cfg.Database.S3.Endpoint)
	}
	svc := usersdomain.NewService(repo, authSvc, storage, log)
	usersHandler := usersdomain.NewHandler(svc, log)

	bootstrapAdmin(log, svc, cfg.Auth.Bootstrap)

	metricsRepo := metricsdomain.NewRepository(pool)
	metricsSvc := metricsdomain.NewService(metricsRepo, log)
	metricsHandler := metricsdomain.NewHandler(metricsSvc, log)

	if cfg.Environment == "local" && !strings.Contains(cfg.Database.Postgres.DSN, "_e2e") {
		bootstrapDevMetrics(log, pool, metricsSvc)
	}

	flagsRepo := flagsdomain.NewRepository(pool)
	experimentsRepo := experimentsdomain.NewRepository(pool)
	refresher := panelsnapshot.NewRefresher(newSnapshotSource(flagsRepo, experimentsRepo), snapWriter, log)
	bootstrapSnapshotRefresh(log, refresher)
	flagsSvc := flagsdomain.NewService(flagsRepo, refresher, log)
	flagsHandler := flagsdomain.NewHandler(flagsSvc, log)

	var experimentsSvc *experimentsdomain.Service
	reviewsRepo := reviewsdomain.NewRepository(pool)
	ownerOf := func(ctx context.Context, experimentID string) (string, error) {
		st, err := experimentsRepo.GetState(ctx, experimentID)
		if err != nil {
			if errors.Is(err, experimentsdomain.ErrNotFound) {
				return "", reviewsdomain.ErrNotFound
			}
			return "", err
		}
		return st.OwnerID, nil
	}
	applyOutcome := func(ctx context.Context, callerID, experimentID, toStatus string, version int) error {
		_, err := experimentsSvc.ApplyReviewOutcome(ctx, callerID, experimentID, experimentsdomain.Status(toStatus), version)
		if err == nil {
			return nil
		}
		switch {
		case errors.Is(err, experimentsdomain.ErrNotFound):
			return reviewsdomain.ErrNotFound
		case errors.Is(err, experimentsdomain.ErrInvalidTransition),
			errors.Is(err, experimentsdomain.ErrVersionConflict):
			return reviewsdomain.ErrReviewClosed
		default:
			return err
		}
	}
	reviewsSvc := reviewsdomain.NewService(reviewsRepo, repo, ownerOf, applyOutcome, log)
	experimentsSvc = experimentsdomain.NewService(experimentsRepo, repo, flagsSvc, reviewsSvc, refresher, log)
	experimentsHandler := experimentsdomain.NewHandler(experimentsSvc, log)
	reviewsHandler := reviewsdomain.NewHandler(reviewsSvc, log)

	authMW, err := authdomain.NewMiddleware(cfg, authSvc, log)
	if err != nil {
		log.Fatal("auth middleware init failed", zap.Error(err))
	}

	anyAuthGroup := apiV1.Group("", authMW.Handler(nil))
	usersHandler.RegisterMeRoute(anyAuthGroup)

	adminGroup := apiV1.Group("", authMW.Handler([]usersdomain.Role{usersdomain.RoleAdmin}))
	usersHandler.RegisterRoutes(adminGroup)

	experimenterGroup := apiV1.Group("", authMW.Handler([]usersdomain.Role{usersdomain.RoleAdmin, usersdomain.RoleExperimenter}))

	flagsHandler.RegisterRoutes(anyAuthGroup)
	flagsHandler.RegisterWriteRoutes(adminGroup)

	metricsHandler.RegisterRoutes(anyAuthGroup)

	experimentsHandler.RegisterRoutes(anyAuthGroup)
	experimentsHandler.RegisterWriteRoutes(experimenterGroup)
	experimentsHandler.RegisterInternalRoutes(adminGroup)

	reviewsHandler.RegisterGroupRoutes(adminGroup)
	reviewsHandler.RegisterReviewRoutes(anyAuthGroup)

	return r, refresher, snapReader
}

func bootstrapAdmin(log *zap.Logger, svc *usersdomain.Service, cfg config.BootstrapConfig) {
	if cfg.PasswordHash == "" {
		log.Warn("auth.bootstrap.password_hash is not set; cannot create the first admin automatically")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	created, err := svc.EnsureBootstrapAdmin(ctx, cfg.FullName, cfg.Email, cfg.PasswordHash)
	if err != nil {
		log.Error("bootstrap admin creation failed", zap.Error(err))
		return
	}

	if !created {
		log.Info("users table is not empty; bootstrap skipped")
	}
}

func bootstrapDevMetrics(log *zap.Logger, pool *pgxpool.Pool, svc *metricsdomain.Service) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var n int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM metrics`).Scan(&n); err != nil {
		log.Warn("dev metrics bootstrap: count failed", zap.Error(err))
		return
	}
	if n > 0 {
		return
	}

	var adminID string
	if err := pool.QueryRow(ctx, `SELECT id FROM users WHERE role='admin' AND deleted_at IS NULL ORDER BY created_at LIMIT 1`).Scan(&adminID); err != nil || adminID == "" {
		log.Warn("dev metrics bootstrap: no admin found, skipping", zap.Error(err))
		return
	}

	samples := []metricsdomain.CreateMetricRequest{
		{
			Key:         "purchase_count",
			Name:        "Purchase Count",
			Description: "Number of purchase events (dev)",
			MetricType:  "count",
			Aggregation: metricsdomain.Aggregation{EventType: strPtr("purchase")},
			Attribution: metricsdomain.Attribution{RequireExposure: true, WindowDays: 7, Fallback: metricsdomain.AttributionFallbackSubject},
		},
		{
			Key:         "revenue_sum",
			Name:        "Revenue Sum",
			Description: "Sum of revenue (dev)",
			MetricType:  "sum",
			Aggregation: metricsdomain.Aggregation{EventType: strPtr("purchase"), Field: strPtr("revenue")},
			Attribution: metricsdomain.Attribution{RequireExposure: true, WindowDays: 7, Fallback: metricsdomain.AttributionFallbackSubject},
		},
		{
			Key:         "conversion_rate",
			Name:        "Conversion Rate",
			Description: "Purchase / visit ratio (dev)",
			MetricType:  "ratio",
			Aggregation: metricsdomain.Aggregation{Numerator: &metricsdomain.EventRef{EventType: "purchase"}, Denominator: &metricsdomain.EventRef{EventType: "visit"}},
			Attribution: metricsdomain.Attribution{RequireExposure: true, WindowDays: 7, Fallback: metricsdomain.AttributionFallbackSubject},
		},
	}

	for _, req := range samples {
		if _, err := svc.Create(ctx, adminID, req); err != nil {
			log.Warn("dev metrics bootstrap: create failed", zap.String("metric.key", req.Key), zap.Error(err))
		} else {
			log.Info("dev metrics bootstrap: metric created", zap.String("metric.key", req.Key))
		}
	}
}

func strPtr(s string) *string {
	return &s
}

type snapshotSource struct {
	flags *flagsdomain.Repository
	exps  *experimentsdomain.Repository
}

func newSnapshotSource(flags *flagsdomain.Repository, exps *experimentsdomain.Repository) panelsnapshot.Source {
	return snapshotSource{flags: flags, exps: exps}
}

func (s snapshotSource) ListFlags(ctx context.Context) ([]snapshot.FlagInput, error) {
	flags, err := s.flags.List(ctx, 10000, 0)
	if err != nil {
		return nil, err
	}
	inputs := make([]snapshot.FlagInput, 0, len(flags))
	for _, f := range flags {
		inputs = append(inputs, snapshot.FlagInput{
			Key:   f.Key,
			Type:  string(f.Type),
			Value: []byte(f.DefaultValue),
		})
	}
	return inputs, nil
}

func (s snapshotSource) ListExperiments(ctx context.Context) ([]snapshot.ExperimentInput, error) {
	running, err := s.exps.ListRunning(ctx)
	if err != nil {
		return nil, err
	}
	inputs := make([]snapshot.ExperimentInput, 0, len(running))
	for _, re := range running {
		variants := make([]snapshot.VariantInput, 0, len(re.Variants))
		for _, v := range re.Variants {
			variants = append(variants, snapshot.VariantInput{
				ID:       v.ID,
				Name:     v.Name,
				Value:    []byte(v.Value),
				WeightBp: v.WeightBP,
			})
		}
		var targeting []byte
		if re.Version.Targeting != nil {
			targeting = []byte(*re.Version.Targeting)
		}
		inputs = append(inputs, snapshot.ExperimentInput{
			ID:           re.Experiment.ID,
			FlagKey:      re.FlagKey,
			VersionNum:   re.Version.VersionNum,
			Salt:         re.Version.DistributionSalt,
			AllocationBp: re.Version.WeightsTotal,
			Targeting:    targeting,
			Variants:     variants,
		})
	}
	return inputs, nil
}

func bootstrapSnapshotRefresh(log *zap.Logger, refresher panelsnapshot.Refresher) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := refresher.RefreshSync(ctx); err != nil {
		log.Warn("snapshot bootstrap refresh failed; will retry on first mutation",
			zap.Error(err),
		)
		return
	}

	log.Info("snapshot bootstrap refresh completed")
}

func addMiddleware(r *gin.Engine, cfg *config.Config, log *zap.Logger) {
	r.Use(middleware.RecoveryGin(log))
	r.Use(middleware.RequestIDGin())
	r.Use(middleware.LoggerGin(log))
	r.Use(corsMiddleware(cfg.Panel.HTTP.CORS))
}

func corsMiddleware(cfg config.CORSConfig) gin.HandlerFunc {
	corsCfg := cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     cfg.AllowedMethods,
		AllowHeaders:     cfg.AllowedHeaders,
		ExposeHeaders:    cfg.ExposeHeaders,
		AllowCredentials: cfg.AllowCredentials,
		MaxAge:           cfg.MaxAge,
	}

	return cors.New(corsCfg)
}
