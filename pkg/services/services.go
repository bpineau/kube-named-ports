// Package services watchs for services annotations.
package services

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/bpineau/kube-named-ports/config"
	"github.com/bpineau/kube-named-ports/pkg/worker"

	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

var (
	maxProcessRetry          = 6
	namedPortNameAnnotation  = "kube-named-ports.io/port-name"
	namedPortValueAnnotation = "kube-named-ports.io/port-value"
)

// Controller are started in a persistent goroutine at program launch,
// and are responsible for watching resources, and for calling worker
// when those resources changes.
type Controller struct {
	conf      *config.KnpConfig
	queue     workqueue.RateLimitingInterface
	informer  cache.SharedIndexInformer
	listWatch cache.ListerWatcher
	stopCh    chan struct{}
	worker    worker.Worker
	wg        *sync.WaitGroup
	initMu    sync.Mutex
	syncInit  bool
}

// NewController creates and initialize the service controller
func NewController(conf *config.KnpConfig, w worker.Worker) *Controller {
	c := &Controller{
		conf:   conf,
		worker: w,
	}

	client := c.conf.ClientSet
	c.listWatch = &cache.ListWatch{
		ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
			return client.CoreV1().Services(meta_v1.NamespaceAll).List(options)
		},
		WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
			return client.CoreV1().Services(meta_v1.NamespaceAll).Watch(options)
		},
	}

	return c
}

// Start initialize and launch a controller. The sync.WaitGroup
// argument is expected to be aknowledged (Done()) at controller
// termination, when Stop() is called.
func (c *Controller) Start(wg *sync.WaitGroup) {
	c.conf.Logger.Infof("Starting services controller")

	c.stopCh = make(chan struct{})

	c.wg = wg

	c.initMu.Lock()
	c.syncInit = true
	c.initMu.Unlock()

	c.startInformer()

	c.worker.Start()
	go c.run(c.stopCh)

	<-c.stopCh
}

// Stop ends a controller and notify the controller's WaitGroup
func (c *Controller) Stop() {
	c.conf.Logger.Infof("Stopping services controller")

	// don't stop while we're still starting
	c.initMu.Lock()
	for !c.syncInit {
		time.Sleep(time.Millisecond)
	}
	c.initMu.Unlock()

	close(c.stopCh)
	c.worker.Stop()

	// give everything 0.2s max to stop gracefully
	time.Sleep(200 * time.Millisecond)

	c.wg.Done()
}

func (c *Controller) startInformer() {
	c.queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	c.informer = cache.NewSharedIndexInformer(
		c.listWatch,
		&core_v1.Service{},
		c.conf.ResyncIntv,
		cache.Indexers{},
	)

	c.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				c.queue.Add(key)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				c.queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				c.queue.Add(key)
			}
		},
	})
}

func (c *Controller) run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	go c.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	c.conf.Logger.Infof("services controller synced and ready")

	wait.Until(c.runWorker, time.Second, stopCh)
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
		// continue looping
	}
}

func (c *Controller) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.processItem(key.(string))

	if err == nil {
		// No error, reset the ratelimit counters
		c.queue.Forget(key)
	} else if c.queue.NumRequeues(key) < maxProcessRetry {
		c.conf.Logger.Errorf("Error processing %s (will retry): %v", key, err)
		c.queue.AddRateLimited(key)
	} else {
		// err != nil and too many retries
		c.conf.Logger.Errorf("Error processing %s (giving up): %v", key, err)
		c.queue.Forget(key)
	}

	return true
}

func (c *Controller) processItem(key string) error {
	obj, _, err := c.informer.GetIndexer().GetByKey(key)

	if err != nil {
		return fmt.Errorf("Error fetching object with key %s from store: %v", key, err)
	}

	svc := obj.(*core_v1.Service)
	portName, ok := svc.Annotations[namedPortNameAnnotation]
	if !ok {
		return nil
	}

	val, res := svc.Annotations[namedPortValueAnnotation]
	if !res {
		return nil
	}

	portValue, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return fmt.Errorf("Failed to parse port value '%s' annotation %v", val, err)
	}

	c.worker.Add(portName, portValue)
	return nil
}
