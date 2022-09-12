package client_go

import (
	"context"
	"fmt"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"testing"
)

func TestClientSet(t *testing.T) {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		inClusterConfig, _ := rest.InClusterConfig()
		config = inClusterConfig
	}

	clientSet, _ := kubernetes.NewForConfig(config)
	deploymentList, _ := clientSet.AppsV1().Deployments("kube-system").List(context.Background(), v1.ListOptions{})
	for _, deployment := range deploymentList.Items {
		fmt.Println(deployment.Name, deployment.Spec.Template.Spec.Containers[0].Image)
	}
}
