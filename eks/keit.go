package main

import (
	"encoding/json"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Define the structure to match the JSON response
type Impacts struct {
	GWP struct {
		Unit        string `json:"unit"`
		Description string `json:"description"`
		Embedded    struct {
			Value    float64  `json:"value"`
			Min      float64  `json:"min"`
			Max      float64  `json:"max"`
			Warnings []string `json:"warnings"`
		} `json:"embedded"`
		Use struct {
			Value float64 `json:"value"`
			Min   float64 `json:"min"`
			Max   float64 `json:"max"`
		} `json:"use"`
	} `json:"gwp"`
}

type Response struct {
	Impacts Impacts `json:"impacts"`
}

// Prometheus metric to track the embedded value per instance type
var (
	embeddedValueGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "eks_node_embedded_value",
			Help: "The embedded value of AWS instance types running in the EKS cluster.",
		},
		[]string{"node_name", "instance_type"},
	)
)

func init() {
	// Register the metric with Prometheus
	prometheus.MustRegister(embeddedValueGauge)
}

// Function to call Boavizta API and get the embedded value
func getEmbeddedValue(instanceType string) (float64, error) {
	url := fmt.Sprintf("http://boavizta-service:5000/v1/cloud/instance?provider=aws&instance_type=%s&verbose=false&duration=43800&criteria=gwp", instanceType)

	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("non-OK HTTP status: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response body: %v", err)
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return 0, fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	return response.Impacts.GWP.Embedded.Value, nil
}

func recordMetrics(clientset *kubernetes.Clientset) {
	// Create a map to store instance type per node to avoid repeated API calls
	nodeInstanceTypeMap := make(map[string]string)
	// Create a map to store embedded values for each instance type
	instanceEmbeddedValueMap := make(map[string]float64)

	// List all pods across all namespaces
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Failed to list pods: %v", err)
	}

	for _, pod := range pods.Items {
		nodeName := pod.Spec.NodeName
		if nodeName == "" {
			fmt.Printf("Pod %s in namespace %s is not scheduled on any node yet.\n", pod.Name, pod.Namespace)
			continue
		}

		instanceType, found := nodeInstanceTypeMap[nodeName]
		if !found {
			node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
			if err != nil {
				log.Printf("Failed to get node %s: %v", nodeName, err)
				continue
			}

			instanceType = node.Labels["node.kubernetes.io/instance-type"]
			nodeInstanceTypeMap[nodeName] = instanceType
		}

		embeddedValue, found := instanceEmbeddedValueMap[instanceType]
		if !found {
			embeddedValue, err = getEmbeddedValue(instanceType)
			if err != nil {
				log.Printf("Failed to get embedded value for instance type %s: %v", instanceType, err)
				continue
			}
			instanceEmbeddedValueMap[instanceType] = embeddedValue
		}

		// Record the metric
		embeddedValueGauge.With(prometheus.Labels{
			"node_name":    nodeName,
			"instance_type": instanceType,
		}).Set(embeddedValue)
		// fmt.Printf("Pod %s in namespace %s is running on node %s which is of AWS instance type: %s with Embedded Value: %f\n",
                //        pod.Name, pod.Namespace, nodeName, instanceType, embeddedValue)
	}
}

func main() {
        fmt.Printf("Starting Keit up\n")

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to create in-cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create clientset: %v", err)
	}

	// Start the metrics server in a goroutine
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	// Run the recordMetrics function periodically
	for {
		recordMetrics(clientset)
		time.Sleep(300 * time.Second)
	}
}

