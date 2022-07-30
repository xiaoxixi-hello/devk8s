package pkg

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsv1 "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"log"
	"time"
)

type controller struct {
	clientset        kubernetes.Interface
	deploymentLister listersv1.DeploymentLister
	deploymentSynced cache.InformerSynced            // deployment的缓存队列
	workQueue        workqueue.RateLimitingInterface // 限速队列
}

func NewController(clientSet kubernetes.Interface, d appsv1.DeploymentInformer) *controller {
	c := &controller{
		clientset:        clientSet,
		deploymentLister: d.Lister(),
		deploymentSynced: d.Informer().HasSynced,                                                                        // 初始化sync
		workQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "shareProcess"), //初始化限速队列
	}

	d.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.queue(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.queue(newObj)
		},
		DeleteFunc: nil,
	})
	return c
}

func (c *controller) queue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("getting key for cache %s\n", err.Error()))
	}
	c.workQueue.Add(key)
}

func (c *controller) Run(stopCh chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workQueue.ShuttingDown()

	log.Println("Starting shareProcessNs controller")
	log.Println("waiting for informer caches to sync")

	if !cache.WaitForCacheSync(stopCh, c.deploymentSynced) {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	for i := 0; i < 10; i++ {

	}
	go wait.Until(c.runWorker, time.Second, stopCh)
	<-stopCh

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the  workqueue.
func (c *controller) runWorker() {
	for c.processNextWorkItem() {
	}

}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *controller) processNextWorkItem() bool {
	item, shutdown := c.workQueue.Get()
	if shutdown {
		return false
	}
	// do obj
	err := func(obj interface{}) error {
		defer c.workQueue.Done(obj)

		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.workQueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		if err := c.syncHandler(key); err != nil {
			c.workQueue.AddRateLimited(key)
			return fmt.Errorf("error syncing %s: %s, requeuing", key, err.Error())
		}
		c.workQueue.Forget(obj)
		return nil
	}(item)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}
	return true
}

func (c *controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	log.Printf("do %s/%s", name, namespace)
	if err != nil {
		return err
	}

	deployment, err := c.deploymentLister.Deployments(namespace).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if _, ok := deployment.GetAnnotations()["shell"]; !ok {
		for i, container := range deployment.Spec.Template.Spec.Containers {
			if container.Name == "shell" {
				deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers[:i],
					deployment.Spec.Template.Spec.Containers[i+1:]...)
				deployment.Spec.Template.Spec.ShareProcessNamespace = boolPtr(false)
				if _, err := c.clientset.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{}); err != nil {
					return err
				}
				return nil
			}
		}
		return nil
	}
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "shell" {
			return nil
		}
	}
	container := corev1.Container{
		Name:  "shell",
		Image: "busybox",
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"SYS_PTRACE"},
			},
		},
		Stdin: true,
		TTY:   true,
	}
	deployment.Spec.Template.Spec.ShareProcessNamespace = boolPtr(true)
	deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, container)
	if _, err := c.clientset.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{}); err != nil {
		return err
	}
	log.Printf("done  ok %s/%s", name, namespace)
	return nil
}
func boolPtr(b bool) *bool { return &b }
