package main

import (
	"context"
	"fmt"
	apps_v1 "k8s.io/api/apps/v1"
	api_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var serverStartTime time.Time

func main() {
	klog.Info("Execute")
	Execute()
}

func Execute() {
	var kubeClient kubernetes.Interface
	ctx := context.Background()

	if _, err := rest.InClusterConfig(); err != nil {
		kubeClient = GetClientOutOfCluster()
	} else {
		kubeClient = GetClient()
	}

	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
				return kubeClient.AppsV1().Deployments(api_v1.NamespaceDefault).List(ctx, options)
			},
			WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
				return kubeClient.AppsV1().Deployments(api_v1.NamespaceDefault).Watch(ctx, options)
			},
		},
		&apps_v1.Deployment{},
		0,
		cache.Indexers{},
	)

	c := NewController(kubeClient, informer)
	klog.Infof("run controller: %v", informer)

	stopCh := make(chan struct{})
	defer close(stopCh)
	go c.Run(stopCh)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	signal.Notify(sigterm, syscall.SIGINT)
	<-sigterm
}

type Controller struct {
	client   kubernetes.Interface
	informer cache.SharedIndexInformer
	queue    workqueue.RateLimitingInterface
}

func NewController(client kubernetes.Interface, informer cache.SharedIndexInformer) *Controller {
	klog.Info("NewController")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// getting access to primitives
	nodes, err := client.CoreV1().Nodes().List(ctx, meta_v1.ListOptions{})
	if err != nil {
		klog.Error(err)
	}
	klog.Infof("nodes: %+v", nodes)

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			klog.Infof("processing add key: %v", key)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(oldObj)
			klog.Infof("processing update key: %v", key)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err != nil {
				klog.Error(err)
			}
			klog.Infof("ignore delete key: %v", key)
		},
	})

	return &Controller{
		client: client,
		queue:  queue,
		informer: informer,
	}
}

func GetClientOutOfCluster() kubernetes.Interface {
	config, err := buildOutOfClusterConfig()
	if err != nil {
		klog.Fatalf("can not get kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("can not get kubernetes config: %v", err)
	}
	return clientset
}

func buildOutOfClusterConfig() (*rest.Config, error) {
	kubeConfigPath := os.Getenv("KUBECONFIG")
	if kubeConfigPath == "" {
		kubeConfigPath = os.Getenv("HOME") + "/.kube/config"
	}
	return clientcmd.BuildConfigFromFlags("", kubeConfigPath)
}

func GetClient() kubernetes.Interface {
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("can not get kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("can not get kubernetes config: %v", err)
	}
	return clientset
}

func (c *Controller) Run(stopCh <-chan struct{}) {
	fmt.Println("running controller")
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Info("Starting sensitive manager controller")
	serverStartTime = time.Now().Local()

	go c.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	klog.Infof("Controller synced and ready: %v", serverStartTime)
	wait.Until(c.runWorker, time.Second, stopCh)
	klog.Info("stopping pod controller")
}

func (c *Controller) HasSynced() bool {
	return c.informer.HasSynced()
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
		// continue loop
	}
}

func (c *Controller) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)
	err := c.processItem(key.(string))
	if err != nil {
		klog.Errorf("Error processing %v", key)
		utilruntime.HandleError(err)
	}

	return true
}

func (c *Controller) processItem(key string) error {
	obj, _, err := c.informer.GetIndexer().GetByKey(key)
	if err != nil {
		return fmt.Errorf("error fetching object with key %s from store: %v", key, err)
	}
	klog.Infof("got obj metadata: %v", obj.(*apps_v1.Deployment).GetLabels())

	return nil
}