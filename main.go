package main

import (
	"os"
	"os/signal"
	"syscall"
	"flag"

	log "github.com/Sirupsen/logrus"
	api_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"

	"github.com/aws/aws-sdk-go/service/sts"
  "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/aws"
)

// retrieve the Kubernetes cluster client from outside of the cluster
func getKubernetesClient() kubernetes.Interface {
	// construct the path to resolve to `~/.kube/config`
	kubeConfigPath := os.Getenv("HOME") + "/.kube/config"

	// create the config from the path
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		log.Fatalf("getClusterConfig: %v", err)
	}

	// generate the client based off of the config
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("getClusterConfig: %v", err)
	}

	log.Info("Successfully constructed k8s client")
	return client
}

var accountID string
var region string

func init() {
	flag.StringVar(&region, "region", "us-east-1", "ACM region")
	flag.Parse()
	log.Info("Region: ", region)

	svc := sts.New(session.New())
	input := &sts.GetCallerIdentityInput{}

	result, err := svc.GetCallerIdentity(input)
	if err != nil {
	    if aerr, ok := err.(awserr.Error); ok {
	        switch aerr.Code() {
	        default:
	            panic(aerr.Error())
	        }
	    } else {
	        panic(err.Error())
	    }
	}
	accountID = *result.Account
}

func main() {
	client := getKubernetesClient()
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
				return client.CoreV1().Secrets("").List(options)
			},
			WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
				return client.CoreV1().Secrets("").Watch(options)
			},
		},
		&api_v1.Secret{},
		0,             // no resync (period of 0)
		cache.Indexers{},
	)

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			log.Infof("Add secret: %s", key)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			log.Infof("Update secret: %s", key)
			if err == nil {
				queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			log.Infof("Delete secret: %s", key)
			if err == nil {
				queue.Add(key)
			}
		},
	})

	controller := Controller{
		logger:    log.NewEntry(log.New()),
		clientset: client,
		informer:  informer,
		queue:     queue,
		handler:   &SecretHandler{
			AccountID: accountID,
			Client: acm.New(session.Must(session.NewSession()), aws.NewConfig().WithRegion(region)),
		},
	}

	stopCh := make(chan struct{})
	defer close(stopCh)
	go controller.Run(stopCh)
	sigTerm := make(chan os.Signal, 1)
	signal.Notify(sigTerm, syscall.SIGTERM)
	signal.Notify(sigTerm, syscall.SIGINT)
	<-sigTerm
}
