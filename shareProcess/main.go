package main

import (
	"github.com/ylinyang/devk8s/shareProcess/pkg"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"time"
)

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		inClusterConfig, err := rest.InClusterConfig()
		if err != nil {
			log.Panicln(err)
		}
		config = inClusterConfig
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Panicln(err)
	}

	sharedInformerFactory := informers.NewSharedInformerFactory(clientset, 1*time.Minute)
	deploymentInformer := sharedInformerFactory.Apps().V1().Deployments()

	ch := make(chan struct{})
	controller := pkg.NewController(clientset, deploymentInformer)

	sharedInformerFactory.Start(ch)
	if err := controller.Run(ch); err != nil {
		log.Panicln(err)
	}
}
