package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/ergoapi/util/exgin"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/ysicing/kubetls/constants"
	"github.com/ysicing/kubetls/pkg/cron"
	"github.com/ysicing/kubetls/pkg/k8s"
	"k8s.io/client-go/informers"
)

func Serve(ctx context.Context) error {
	defer cron.Cron.Stop()
	cron.Cron.Start()
	g := exgin.Init(&exgin.Config{
		Debug:   true,
		Cors:    true,
		Metrics: true,
	})
	g.Use(exgin.ExTraceID())
	g.Use(exgin.ExLog())
	g.Use(exgin.ExRecovery())
	g.GET("/version", func(c *gin.Context) {
		exgin.GinsData(c, map[string]string{
			"builddate": constants.Date,
			"release":   constants.Release,
			"gitcommit": constants.Commit,
			"version":   constants.Version,
			"k8s":       k8s.KV(),
		}, nil)
	})
	g.GET("/check/:id", func(c *gin.Context) {
		id := exgin.GinsParamStr(c, "id")
		exgin.GinsData(c, nil, k8s.Check(id))
	})
	g.NoMethod(func(c *gin.Context) {
		msg := fmt.Sprintf("not found: %v", c.Request.Method)
		exgin.GinsAbortWithCode(c, 404, msg)
	})
	g.NoRoute(func(c *gin.Context) {
		msg := fmt.Sprintf("not found: %v", c.Request.URL.Path)
		exgin.GinsAbortWithCode(c, 404, msg)
	})
	cron.Cron.Add("@every 60s", func() {
		k8s.Pods()
	})
	stopChan := make(chan struct{})
	factory := informers.NewSharedInformerFactory(k8s.K8SClient, time.Hour)
	controller := k8s.NewNamespaceControlller(factory)
	controller.Run(stopChan)
	addr := "0.0.0.0:65001"
	srv := &http.Server{
		Addr:    addr,
		Handler: g,
	}
	go func() {
		defer close(stopChan)
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.TODO(), time.Second*5)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logrus.Errorf("Failed to stop server, error: %s", err)
		}
		logrus.Info("server exited.")
	}()
	logrus.Infof("http listen to %v, pid is %v", addr, os.Getpid())
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logrus.Errorf("Failed to start http server, error: %s", err)
		return err
	}

	<-stopChan

	return nil
}
