package main

import (
	"context"
	"errors"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"log"
	"math/rand"
)

const schedulerName = "random-scheduler"

var m = make(map[string]string)

type predicateFunc func(node *v1.Node, pod *v1.Pod) bool
type priorityFunc func(node *v1.Node, pod *v1.Pod) int

type SchedulerTest struct {
	client     *kubernetes.Clientset
	podQueue   chan *v1.Pod
	nodeLister corev1.NodeLister
	predicates []predicateFunc
	priorities []priorityFunc
}

func NewScheduler(c *kubernetes.Clientset, podQueue chan *v1.Pod, quit chan struct{}) SchedulerTest {
	return SchedulerTest{
		client:     c,
		podQueue:   podQueue,
		nodeLister: initInformers(c, podQueue, quit),
		predicates: []predicateFunc{
			randomPredicate,
		},
		priorities: []priorityFunc{
			randomPriority,
		},
	}
}

func (s *SchedulerTest) Run(quit chan struct{}) {
	wait.Until(s.SchedulerOne, 0, quit)
}

func (s *SchedulerTest) SchedulerOne() {
	p := <-s.podQueue
	log.Println("当前调度的pod是：", p.Namespace, "/", p.Name)
	//	fmt.Println("found a pod a schedule:", p.Namespace, "/", p.Name)

	log.Println("开始选择最佳节点：")
	node, err := s.findFit(p)
	if err != nil {
		log.Println("cannot find node that fits pod", err.Error())
		return
	}
	log.Println("绑定pod到node上：", p.Namespace, "/", p.Name, "/", node)
	if err := s.bindPod(p, node); err != nil {
		log.Println("failed to bind pod", err.Error())
		return
	}

	// todo 事件通知函数
}

// 从预选队列里面 选出最优的node
func (s *SchedulerTest) findFit(pod *v1.Pod) (string, error) {
	// Everything returns a selector that matches all labels.
	nodes, err := s.nodeLister.List(labels.Everything())
	if err != nil {
		return "", err
	}
	n := []string{}
	for _, node := range nodes {
		n = append(n, node.Name)
	}
	log.Println("集群中的node：", n)
	log.Println("开始预选node: ")
	filteredNodes := s.runPredicates(nodes, pod)

	m := []string{}
	for _, node := range filteredNodes {
		m = append(m, node.Name)
	}
	log.Println("预选出来的node是：", m)
	if len(filteredNodes) == 0 {
		return "", errors.New("failed to find node that fits pod")
	}

	log.Println("开始对node进行打分：")
	priorities := s.prioritize(filteredNodes, pod)
	log.Println("开始选择最优的node: ")
	log.Println(priorities)
	return s.findBestNode(priorities), nil
}

// 查看那些node能够进入预选队列里面
func (s *SchedulerTest) runPredicates(nodes []*v1.Node, pod *v1.Pod) []*v1.Node {
	filteredNodes := make([]*v1.Node, 0)
	// 循环整个nodeList
	for _, node := range nodes {
		if s.predicatesApply(node, pod) {
			filteredNodes = append(filteredNodes, node)
		}
	}
	//for _, node := range filteredNodes {
	//	log.Println(node.Name)
	//}
	return filteredNodes
}

func (s *SchedulerTest) predicatesApply(node *v1.Node, pod *v1.Pod) bool {
	// 查预选队列 是否有满足的
	for _, predicate := range s.predicates {
		if !predicate(node, pod) {
			return false
		}
	}
	return true
}

// 针对node打分
func (s *SchedulerTest) prioritize(nodes []*v1.Node, pod *v1.Pod) map[string]int {
	priorities := make(map[string]int) // 打分
	for _, node := range nodes {
		for _, priority := range s.priorities { // 调用随机函数进行打分的
			priorities[node.Name] += priority(node, pod)
		}
	}
	fmt.Println(priorities)
	return priorities
}

// 比较最大的分数的节点
func (s *SchedulerTest) findBestNode(predicates map[string]int) string {
	var maxP int
	var bestNode string
	for node, p := range predicates {
		if p > maxP {
			maxP = p
			bestNode = node
		}
	}
	return bestNode
}

func (s *SchedulerTest) bindPod(p *v1.Pod, node string) error {
	m[node] = p.Name
	return s.client.CoreV1().Pods(p.Namespace).Bind(context.Background(), &v1.Binding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.Name,
			Namespace: p.Namespace,
		},
		Target: v1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Node",
			Name:       node,
		},
	}, metav1.CreateOptions{})
}

func initInformers(c *kubernetes.Clientset, podQueue chan *v1.Pod, quit chan struct{}) corev1.NodeLister {
	factory := informers.NewSharedInformerFactory(c, 0)

	nodeInformer := factory.Core().V1().Nodes()
	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			node, ok := obj.(*v1.Node)
			if !ok {
				log.Println("this is not a node")
				return
			}
			log.Printf("new node add to store:%s", node.GetName())
		},
	})

	podInformer := factory.Core().V1().Pods()
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod, ok := obj.(*v1.Pod)
			if !ok {
				log.Println("this is a not a pod")
				return
			}
			if pod.Spec.NodeName == "" && pod.Spec.SchedulerName == schedulerName {
				podQueue <- pod
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			if pod.Spec.SchedulerName == schedulerName {
				delete(m, pod.Spec.NodeName)
			}
		},
	})

	factory.Start(quit)
	return nodeInformer.Lister()
}

// 预选算法
func randomPredicate(node *v1.Node, pod *v1.Pod) bool {
	_ = rand.Intn(2)
	_, ok := m[node.Name]
	if ok {
		return false
	}
	return true
}

// 优选算法
func randomPriority(node *v1.Node, pod *v1.Pod) int {
	return rand.Intn(100)
}
