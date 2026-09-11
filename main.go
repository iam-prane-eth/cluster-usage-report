package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"sort"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

const (
	configMapName = "monthly-metrics"
)

type Receiver struct {
	K8sClient          *Client
	Namespace          string `envconfig:"NAMESPACE" default:"tcl-security"`
	ConfigPath         string `envconfig:"CONFIG" default:"/etc/config"`
	ReportsPath        string `envconfig:"REPORTS" default:"/etc/reports/cluster-usage-report"`
	ServiceAccountName string `envconfig:"SERVICE_ACCOUNT_NAME" default:"cluster-usage-report-sa"`
	PrometheusAddress  string `envconfig:"PROMETHEUS_ADDRESS"`
	OPENAI_BASE_URL    string `envconfig:"OPENAI_BASE_URL"`
	OPENAI_MODEL       string `envconfig:"OPENAI_MODEL"`
}

type Payload struct {
	Name string `json:"name"`
	File string `json:"file"`
}

func main() {
	protocol, err := cloudevents.NewHTTP()
	if err != nil {
		log.Fatalf("failed to create protocol: %s", err.Error())
	}

	client, err := cloudevents.NewClient(protocol)
	if err != nil {
		log.Fatal(err.Error())
	}

	r := Receiver{}
	if err := envconfig.Process("", &r); err != nil {
		log.Fatal(err.Error())
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		msg := fmt.Errorf("failed to get kubernetes config: %s", err)
		log.Fatal(msg.Error())
	}

	k8sClient, err := NewClient(config, r.Namespace, r.ServiceAccountName)
	if err != nil {
		msg := fmt.Errorf("failed to setup kubernetes client: %s", err)
		log.Fatal(msg.Error())
	}

	r.K8sClient = k8sClient

	log.Print("report generator service is running")
	if err := client.StartReceiver(context.Background(), r.ReceiveAndGenerate); err != nil {
		log.Fatal(err)
	}
}

// ReceiveAndScan is invoked whenever we receive an event.
func (r *Receiver) ReceiveAndGenerate(ctx context.Context, inputEvent event.Event) (*event.Event, protocol.Result) {
	log.Print("recieve event triggered")
	payload := Payload{
		Name: "cluster-usage-report",
	}
	var source struct {
		Trigger string `json:"trigger"`
	}
	inputEvent.DataAs(&source)

	isPingSource := source.Trigger == "PingSource"
	log.Printf("Is PingSource? %v", isPingSource)
	outputEvent := inputEvent.Clone()
	outputEvent.SetType("http")
	outputEvent.SetData(cloudevents.ApplicationJSON, payload)

	client, err := api.NewClient(api.Config{
		Address: r.PrometheusAddress,
	})
	if err != nil {
		log.Fatalf("Error creating Prometheus client (address: %s): %v", r.PrometheusAddress, err)
	}

	v1api := v1.NewAPI(client)
	queries := []string{
		"((sum(node_memory_MemTotal_bytes) - sum(min_over_time(node_memory_MemAvailable_bytes[24h]))) / sum(node_memory_MemTotal_bytes)) * 100",
		"100*(1 - avg(rate(node_cpu_seconds_total{mode=\"idle\"}[24h])))",
		"(sum(max_over_time(kubelet_volume_stats_used_bytes[24h])) / (sum(max_over_time(kubelet_volume_stats_used_bytes[24h])) + sum(min_over_time(kubelet_volume_stats_available_bytes[24h])))) * 100",
		"max_over_time(etcd_server_has_leader[24h])",
		"max_over_time(etcd_server_leader_changes_seen_total[24h])",
		"max_over_time(etcd_mvcc_db_total_size_in_bytes[24h])",
		"sum(max_over_time(apiserver_request_total{group=\"admissionregistration.k8s.io\",verb=\"WATCH\"}[24h]))",
		"sum(max_over_time(apiserver_request_duration_seconds_count{group=\"admissionregistration.k8s.io\",verb=\"WATCH\"}[24h]))",
		"sum(max_over_time(apiserver_response_sizes_count{group=\"admissionregistration.k8s.io\",verb=\"WATCH\"}[24h]))",
		"sum(max_over_time(controller_runtime_reconcile_total[24h]))",
		"sum(max_over_time(controller_runtime_reconcile_errors_total[24h]))",
		"sum(max_over_time(controller_runtime_reconcile_time_seconds_sum[24h]))",
		"sum(max_over_time(scheduler_e2e_scheduling_duration_seconds_sum[24h]))",
		"sum(max_over_time(scheduler_schedule_attempts_total[24h]))",
		"sum(max_over_time(container_cpu_usage_seconds_total[24h]))",
		"sum(max_over_time(container_memory_usage_bytes[24h]))",
		"sum(max_over_time(container_fs_usage_bytes[24h]))",
		"sum(max_over_time(container_memory_working_set_bytes[24h]))",
		"sum(max_over_time(container_memory_max_usage_bytes[24h]))",
		"sum(max_over_time(container_network_receive_bytes_total[24h]))",
		"sum(max_over_time(container_network_transmit_bytes_total[24h]))",
		"sum(max_over_time(container_network_receive_errors_total[24h]))",
		"sum(max_over_time(container_network_transmit_errors_total[24h]))",
		"sum(max_over_time(kubelet_runtime_operations_total[24h]))",
		"sum(max_over_time(kubelet_runtime_operations_errors_total[24h]))",
		"sum(max_over_time(kubelet_pod_start_duration_seconds_count[24h]))",
		"sum(max_over_time(kubelet_pod_worker_duration_seconds_count[24h]))",
		"sum(kube_node_status_condition{condition=\"Ready\", status=\"true\"} == 1)",
	}

	results := make(map[string]float64)
	timestamp := time.Now()
	successCount := 0
	for _, query := range queries {
		result, _, err := v1api.Query(context.Background(), query, timestamp)
		if err != nil {
			log.Printf("Prometheus query error [%s] at [%s]: %v", query, r.PrometheusAddress, err)
			continue
		}

		if vector, ok := result.(model.Vector); ok && len(vector) > 0 {
			for _, sample := range vector {
				results[query] = float64(sample.Value)
				successCount++
			}
		}
	}
	if successCount == 0 {
		log.Printf("All Prometheus queries failed. Check the Prometheus server address or query correctness.")
	}
	if len(results) == 0 {
		log.Printf("No usable Prometheus data collected,writing empty data to ConfigMap.")
	}
	// Convert results to JSON
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		log.Fatalf("Error marshalling results: %v", err)
	}

	configMapKey := fmt.Sprintf("%s-%02d-%d", timestamp.Month().String()[:3], timestamp.Day(), timestamp.Year())

	configMapList, err := r.K8sClient.clientset.CoreV1().ConfigMaps(r.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Unable to list the cm:%v", err)
		return nil, err
	}
	today := time.Now()
	year, month, day := today.Date()
	var flag int64
	for _, cm := range configMapList.Items {
		//fmt.Println(cm.Name)
		if cm.Name == configMapName {
			flag = 1
			log.Printf("ConfigMap %s found in the cluster\n", configMapName)
			break
		}
	}
	if flag != 1 {
		log.Printf("ConfigMap %s not found, creating a new one...\n", configMapName)
		r.CreateConfigMap(configMapName, jsonData, configMapKey)
	}
	if isFirstDayOfMonth(today) {
		log.Printf("Today is the first day of the month, deleting the ConfigMap for fresh data...\n")
		err := r.K8sClient.clientset.CoreV1().ConfigMaps(r.Namespace).Delete(context.TODO(), configMapName, metav1.DeleteOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				log.Printf("ConfigMap %s not found, creating a new one...\n", configMapName)
			} else {
				log.Fatalf("Error deleting oldConfigMap during 1st day of the month for fresh data: %v", err)
				return nil, err
			}
		}
		r.CreateConfigMap(configMapName, jsonData, configMapKey)
	}

	configMap, err := r.K8sClient.clientset.CoreV1().ConfigMaps(r.Namespace).Get(context.TODO(), configMapName, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Failed to fetch ConfigMap %s: %v", configMapName, err)
	}
	// Update existing ConfigMap by adding a new day's data
	configMap.Data[configMapKey] = string(jsonData)
	_, err = r.K8sClient.clientset.CoreV1().ConfigMaps(r.Namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
	if err != nil {
		log.Fatalf("Error updating ConfigMap: %v\n", err)
	}

	log.Printf("Updated ConfigMap %s with new key: %s\n", configMapName, configMapKey)

	var metricsData map[string]map[string]float64

	if isPingSource {
		metricsData, err = r.fetchAllMetricsFromConfigMap(configMapName, r.Namespace, true, 32, month, year)
	} else {
		metricsData, err = r.fetchAllMetricsFromConfigMap(configMapName, r.Namespace, true, day, month, year)
	}

	if err != nil {
		log.Fatalf("Failed to fetch historical metrics from ConfigMap: %v", err)
	}
	log.Println("Successfully fetched previous metrics data")

	extractedMetrics := make(map[string]map[string]float64)

	for date, metrics := range metricsData {
		relevantMetrics := map[string]float64{
			"cpu_usage":       metrics["100*(1 - avg(rate(node_cpu_seconds_total{mode=\"idle\"}[24h])))"],
			"memory_usage":    metrics["((sum(node_memory_MemTotal_bytes) - sum(min_over_time(node_memory_MemAvailable_bytes[24h]))) / sum(node_memory_MemTotal_bytes)) * 100"],
			"pvc_utilization": metrics["(sum(max_over_time(kubelet_volume_stats_used_bytes[24h])) / (sum(max_over_time(kubelet_volume_stats_used_bytes[24h])) + sum(min_over_time(kubelet_volume_stats_available_bytes[24h])))) * 100"],
			"nodes":           metrics["sum(kube_node_status_condition{condition=\"Ready\", status=\"true\"} == 1)"],
		}
		extractedMetrics[date] = relevantMetrics
	}

	jsonData, err = json.MarshalIndent(extractedMetrics, "", "  ") // Pretty-print JSON
	if err != nil {
		log.Fatalf("error marshelling")
	}
	/*
		graph_file := fmt.Sprintf("%s/%s", r.ReportsPath, "input_for_graph.json")
		err = ioutil.WriteFile(graph_file, jsonData, 0644) // Save to file
		if err != nil {
		fmt.Println("error writing file",err)
		}
	*/

        //log.Println("Extracted metrics being used for graph:\n", string(jsonData))

	generateGraph(extractedMetrics, r.ReportsPath)

	if (isLastDayOfMonth(today) && isPingSource) || (!isPingSource) {
		if isPingSource {
			log.Println("Today is the last day of the month.Calculating average cpu,memory,pvc utilization")
		} else {
			log.Println("Calculating average cpu,memory,pvc utilization")
		}
		avgCpuUsage, avgMemoryUsage, avgPvcUsage := CalculateAverageUsages(extractedMetrics)
		metricsData["averages"] = map[string]float64{
			"avgCPU": avgCpuUsage,
			"avgMem": avgMemoryUsage,
			"avgPVC": avgPvcUsage,
		}
		clustername, err := r.GetClusterName()
		if err != nil {
			log.Printf("Warning: Failed to get cluster name: %v", err)
			clustername = "unknown"
		}
		API_Key_Secret,err:=r.K8sClient.clientset.CoreV1().Secrets(r.Namespace).Get(context.TODO(),"openapikey",metav1.GetOptions{})
		if err!=nil{
			if errors.IsNotFound(err){
				log.Fatalf("secret %s not found in %s namespace","openapikey",r.Namespace)
			}
			log.Fatalf("Unable to get secret %s/%s, %v","openapikey",r.Namespace,err)
		}
		apiKeyBytes, ok := API_Key_Secret.Data["key"]
		if !ok {
			log.Fatalf("API key not found in secret under 'key'")
		}

		apiKey := string(apiKeyBytes)
		log.Println("Sending data to OpenAI & generating PDF for cluster %v", clustername)
		analysis, err := analyzeWithOpenAI(metricsData,r.OPENAI_BASE_URL,r.OPENAI_MODEL,apiKey)
		if err != nil {
			log.Fatalf("Error analyzing with OpenAI: %v\n", err)
		}
		reportFilePath := fmt.Sprintf("%s/reports-%s.md", r.ReportsPath, time.Now().Format("2006-01-02"))

		if err := ioutil.WriteFile(reportFilePath, []byte(analysis), 0644); err != nil {
			fmt.Printf("Error writing analysis to file: %v", err)
		}
		pdfFilePath := fmt.Sprintf("%s/report-%s.pdf", r.ReportsPath, time.Now().Format("2006-01-02"))

		//mergeIntoPdf(analysis, pdfFilepath, clustername, isPingSource, day)
                if err := ConvertMarkdownToPDF(reportFilePath, pdfFilePath, clustername, isPingSource, day); err != nil {
                     log.Fatalf("PDF conversion failed: %v", err)
                }

		payloadPath := fmt.Sprintf("report-%s.pdf", time.Now().Format("2006-01-02"))
		payload.File = payloadPath
		if err := outputEvent.SetData(*event.StringOfApplicationJSON(), payload); err != nil {
			msg := fmt.Sprintf("failed to set output data: %s", err)
			log.Print(msg)
			return &outputEvent, http.NewResult(500, msg)
		}

		log.Printf("Transform the event to: [%s] %s %s: %+v", outputEvent.Time(), outputEvent.Source(), outputEvent.Type(), payload)

		return &outputEvent, nil
	}
	if isPingSource {
		log.Println("Not the last day of the month. Skipping Notify...")
	}
	return nil, http.NewResult(204, "No content to send")
}

func CalculateAverageUsages(metricsData map[string]map[string]float64) (float64, float64, float64) {
	var totalPVC, totalMemory, totalCPU float64
	var count int

	for _, dailyMetrics := range metricsData {
		if pvc, ok := dailyMetrics["pvc_utilization"]; ok {
			totalPVC += pvc
		}
		if cpu, ok := dailyMetrics["cpu_usage"]; ok {
			totalCPU += cpu
		}
		if mem, ok := dailyMetrics["memory_usage"]; ok {
			totalMemory += mem
		}
		count++
	}

	if count == 0 {
		return 0, 0, 0
	}

	return totalCPU / float64(count), totalMemory / float64(count), totalPVC / float64(count)
}

func (r *Receiver) GetClusterName() (string, error) {
	nodes, err := r.K8sClient.clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil || len(nodes.Items) == 0 {
		return "", fmt.Errorf("failed to list nodes: %v", err)
	}

	for _, node := range nodes.Items {
		if val, ok := node.Annotations["cluster.x-k8s.io/cluster-name"]; ok {
			fmt.Println("cluster name fetched successfully", val)
			return val, nil
		}
	}

	return "", fmt.Errorf("cluster name annotation not found")
}
func isLastDayOfMonth(date time.Time) bool {
	// Get tomorrow’s date
	tomorrow := date.AddDate(0, 0, 1)
	//fmt.Printf("Tomorrow's date is %s\n",tomorrow.Format("2006-01-02"))
	// If tomorrow's month is different, today is the last day
	return tomorrow.Month() != date.Month()
}

func isFirstDayOfMonth(date time.Time) bool {
	// First day of the month is always 1st
	//fmt.Printf("checking if isFirstDayOfMonth working %v\n",date.Day())
	return date.Day() == 1
}

func (r *Receiver) CreateConfigMap(configMapName string, jsonData []byte, configMapKey string) {
	newConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: r.Namespace,
		},
		Data: map[string]string{
			configMapKey: string(jsonData),
		},
	}
	_, err := r.K8sClient.clientset.CoreV1().ConfigMaps(r.Namespace).Create(context.TODO(), newConfigMap, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("Error creating ConfigMap: %v", err)
	} else {
		log.Printf("Created ConfigMap %s with initial data.\n", configMapName)
		//return
	}
}

func (r *Receiver) fetchAllMetricsFromConfigMap(configMapName string, namespace string, shouldFilter bool, dayLimit int, month time.Month, year int) (map[string]map[string]float64, error) {
	cm, err := r.K8sClient.clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), configMapName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap %q in namespace %q: %v", configMapName, namespace, err)
	}

	aggregatedMetrics := make(map[string]map[string]float64)

	// Sort keys (to ensure chronological order)
	keys := make([]string, 0, len(cm.Data))
	for key := range cm.Data {
		keys = append(keys, key)
	}
	sort.Strings(keys) // Ensures metrics are in date order

	for _, key := range keys {
		parsedDate, err := time.Parse("Jan-02-2006", key)
		if err != nil {
			log.Printf("Skipping invalid date key in CM: %s", key)
			continue
		}

		if shouldFilter {
			if parsedDate.Year() != year || parsedDate.Month() != month || parsedDate.Day() > dayLimit {
				continue
			}
		}
		var metrics map[string]float64
		err = json.Unmarshal([]byte(cm.Data[key]), &metrics)
		if err != nil {
			log.Printf("Skipping %s due to JSON parsing error: %v", key, err)
			continue
		}
		aggregatedMetrics[key] = metrics
	}

	return aggregatedMetrics, nil

}
