package kubernetes

import (
	"context"
	"flag"
	"fmt"
	"log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const DefaultKubeConfigPath = "/home/hitesh/.kube/config"

func GetClusterClient() (*kubernetes.Clientset, error) {
	var kubeconfig string
	if flag.Lookup("kubeconfig") == nil {
		cnf := flag.String("kubeconfig", "/home/hitesh/.kube/config", "location of your kubeconfig file")
		kubeconfig = *cnf
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Printf("error while building config from kubeconfig file location : %s\n", err.Error())
		log.Println("fetching config within cluster")
		config, err = rest.InClusterConfig()
		if err != nil {
			fmt.Printf("error while getting inclusterconfig : %s\n", err.Error())
			return nil, fmt.Errorf("error while creating clusterconfig : %s", err.Error())
		}
	}

	return kubernetes.NewForConfig(config)
}

func GetServiceDnsName(clientset *kubernetes.Clientset, serviceName, namespace string) string {
	service, err := clientset.CoreV1().Services(namespace).Get(context.Background(), serviceName, metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}
	var hostname string
	if len(service.Status.LoadBalancer.Ingress) > 0 {
		hostname = service.Status.LoadBalancer.Ingress[0].Hostname
	} else {
		fmt.Printf("Service %s in namespace %s has no external IP address or hostname\n", serviceName, namespace)
	}

	if hostname == "" {
		hostname = service.Name + "." + service.Namespace + ".svc.cluster.local"
	}

	return fmt.Sprintf("dns:///%s", hostname)
}

func GetEndpointsOfService(clientset *kubernetes.Clientset, serviceName, namespace string) []string {
	endpoints, err := clientset.CoreV1().Endpoints(namespace).Get(context.Background(), serviceName, metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	addresses := make([]string, 0)
	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			addresses = append(addresses, address.IP)
		}
	}

	return addresses
}

func GetPodIpByPodName(clientset *kubernetes.Clientset, podName, namespace string) string {
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	return pod.Status.PodIP
}
