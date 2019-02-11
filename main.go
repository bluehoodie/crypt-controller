// +build !windows

package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	log "k8s.io/klog"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/bluehoodie/crypt-controller/controller"
	clientset "github.com/bluehoodie/crypt-controller/pkg/client/clientset/versioned"
	informers "github.com/bluehoodie/crypt-controller/pkg/client/informers/externalversions"
	"github.com/bluehoodie/crypt-controller/pkg/store/factory"
)

var (
	masterURL   string
	kubeConfig  string
	storeType   string
	storeConfig string
)

func init() {
	flag.StringVar(&kubeConfig, "kubeConfig", os.Getenv("KUBECONFIGPATH"), "Path to a kubeConfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeConfig. Only required if out-of-cluster.")

	flag.StringVar(&storeType, "storeType", os.Getenv("STORE_TYPE"), "The type of store to use a secret source.")
	flag.StringVar(&storeConfig, "storeConfig", os.Getenv("STORE_CONFIG"), "Path to a store config.")
}

func main() {
	flag.Parse()

	stop := make(chan struct{})
	go func() {
		signalStream := make(chan os.Signal)
		signal.Notify(signalStream, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)

		sig := <-signalStream
		log.Warningf("received signal: %v. shutting down.", sig)

		close(stop)

		<-signalStream
		os.Exit(1)
	}()

	if storeType == "" {
		log.Fatal("STORE_TYPE not defined")
	}

	store, err := factory.NewStoreFactory(storeConfig).Make(storeType)
	if err != nil {
		log.Fatalf("Could not initialize store: %v", err)
	}

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeConfig)
	if err != nil {
		log.Fatalf("Error building kubeConfig: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Error building kubernetes clientset: %v", err)
	}

	cryptClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Error building Crypt clientset: %v", err)
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, 30*time.Second)
	cryptInformerFactory := informers.NewSharedInformerFactory(cryptClient, 30*time.Second)

	c := controller.New(kubeClient, cryptClient,
		kubeInformerFactory.Core().V1().Namespaces(),
		kubeInformerFactory.Core().V1().Secrets(),
		cryptInformerFactory.Core().V1alpha1().Crypts(),
		store,
	)

	kubeInformerFactory.Start(stop)
	cryptInformerFactory.Start(stop)

	err = c.Run(3, stop)
	if err != nil {
		log.Fatal(err)
	}
}
