// Package run implements the main loop, by launching the healthcheck service
// and the services' controller loop.
package run

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/mirakl/kube-named-ports/config"
	"github.com/mirakl/kube-named-ports/pkg/health"
	"github.com/mirakl/kube-named-ports/pkg/services"
	"github.com/mirakl/kube-named-ports/pkg/worker"
)

// Run launchs the effective services controllers goroutines
func Run(config *config.KnpConfig) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	defer wg.Wait()

	wrk := worker.NewWorker(config)
	svc := services.NewController(config, wrk)
	go svc.Start(&wg)
	defer func(s *services.Controller) {
		go s.Stop()
	}(svc)

	go func() {
		if err := health.HeartBeatService(config); err != nil {
			config.Logger.Warningf("Healtcheck service failed: %s", err)
		}
	}()

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	signal.Notify(sigterm, syscall.SIGINT)
	<-sigterm

	config.Logger.Infof("Stopping the service controller")
}
