package main

import (
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"math/rand"
	"time"
)

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		clusterConfig, err := rest.InClusterConfig()
		if err != nil {
			log.Panicln(err)
		}
		config = clusterConfig
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Panicln(err)
	}

	fmt.Println("I'm a scheduler!")

	rand.Seed(time.Now().Unix())

	podQueue := make(chan *v1.Pod, 300)
	defer close(podQueue)

	quit := make(chan struct{})
	defer close(quit)

	scheduler := NewScheduler(clientset, podQueue, quit)
	scheduler.Run(quit)
}
