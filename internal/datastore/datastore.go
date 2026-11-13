package datastore

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/padok-team/burrito/internal/burrito/config"
	"github.com/padok-team/burrito/internal/datastore/api"
	"github.com/padok-team/burrito/internal/datastore/storage"
	"github.com/padok-team/burrito/internal/utils/authz"
	log "github.com/sirupsen/logrus"
)

const (
	DefaultCertPath = "/etc/burrito/tls/tls.crt"
	DefaultKeyPath  = "/etc/burrito/tls/tls.key"
)

type Datastore struct {
	Config *config.Config
	API    *api.API
}

func New(c *config.Config) *Datastore {
	return &Datastore{
		Config: c,
	}
}

func (s *Datastore) Exec() {
	if s.Config.Datastore.LogFormat == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	}
	s.API = api.New(s.Config)
	s.API.Storage = storage.New(*s.Config)
	authz := authz.NewAuthz()
	for _, sa := range s.Config.Datastore.AuthorizedServiceAccounts {
		l := strings.Split(sa, "/")
		authz.AddServiceAccount(l[0], l[1])
	}
	authz.SetAudience("burrito")
	log.Infof("starting burrito datastore...")
	e := echo.New()
	e.GET("/healthz", handleHealthz)

	api := e.Group("/api")
	api.Use(middleware.RequestLoggerWithConfig(getLoggerConfig()))
	api.Use(authz.Process)
	api.GET("/logs", s.API.GetLogsHandler)
	api.PUT("/logs", s.API.PutLogsHandler)
	api.GET("/plans", s.API.GetPlanHandler)
	api.PUT("/plans", s.API.PutPlanHandler)
	api.GET("/stategraph", s.API.GetStateGraphHandler)
	api.PUT("/stategraph", s.API.PutStateGraphHandler)
	api.GET("/state", s.API.GetTerraformStateHandler)
	api.POST("/state", s.API.PutTerraformStateHandler)
	api.POST("/state/lock", s.API.LockTerraformStateHandler)
	api.GET("/state/lock", s.API.GetTerraformStateLockHandler)
	api.DELETE("/state/lock", s.API.UnlockTerraformStateHandler)
	api.PUT("/repository/revision/bundle", s.API.PutGitBundleHandler)
	api.GET("/repository/revision/bundle", s.API.GetGitBundleHandler)
	api.HEAD("/repository/revision/bundle", s.API.HeadGitBundleHandler)
	api.POST("/encrypt", s.API.EncryptAllFilesHandler)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	sc := echo.StartConfig{Address: s.Config.Datastore.Addr}
	var err error
	if s.Config.Datastore.TLS {
		cert, readErr := os.ReadFile(DefaultCertPath)
		if readErr != nil {
			err = fmt.Errorf("could not read datastore TLS certificate: %w", readErr)
		} else {
			key, readErr := os.ReadFile(DefaultKeyPath)
			if readErr != nil {
				err = fmt.Errorf("could not read datastore TLS private key: %w", readErr)
			} else {
				err = sc.StartTLS(ctx, e, cert, key)
			}
		}
	} else {
		err = sc.Start(ctx, e)
	}
	if err != nil {
		log.Fatalf("datastore stopped: %v", err)
	}
}

func handleHealthz(c *echo.Context) error {
	return c.String(http.StatusOK, "OK")
}

func getLoggerConfig() middleware.RequestLoggerConfig {
	return middleware.RequestLoggerConfig{
		LogRemoteIP:      true,
		LogMethod:        true,
		LogURI:           true,
		LogStatus:        true,
		LogLatency:       true,
		LogContentLength: true,
		LogResponseSize:  true,
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			user, _ := c.Get("serviceAccount").(string)
			log.WithFields(log.Fields{
				"service_account": user,
				"remote_ip":       v.RemoteIP,
				"method":          v.Method,
				"uri":             v.URI,
				"status":          v.Status,
				"error":           v.Error,
				"latency":         v.Latency.String(),
				"bytes_in":        v.ContentLength,
				"bytes_out":       v.ResponseSize,
			}).Info()
			return nil
		},
	}
}
