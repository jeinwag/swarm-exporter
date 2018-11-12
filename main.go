package main

import (
	"flag"
	"fmt"
	"github.com/docker/docker/api/types/swarm"
	"github.com/fsouza/go-dockerclient"
	"github.com/prometheus/client_golang/prometheus"
	"log"
	"net/http"
)

const (
	namespace = "swarm"
)

var (
	swarmTasks = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "task"),
		"Swarm task",
		[]string{"service_name", "node_id", "state", "task_id"},
		nil,
	)
	swarmServiceReplicas = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "service_replicas"),
		"Swarm service replica count",
		[]string{"service_name"},
		nil,
	)
	swarmServiceRunningReplicas = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "service_running_replicas"),
		"Swarm service running replica count",
		[]string{"service_name"},
		nil,
	)
	swarmNodes = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "node"),
		"Swarm node",
		[]string{"node_id", "hostname", "state", "availability"},
		nil,
	)
	swarmServiceUpdateState = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "service_update_state"),
		"State of last service update",
		[]string{"service_name", "state"},
		nil,
	)
	swarmServiceLastUpdate = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "service_last_update"),
		"Time of last service update",
		[]string{"service_name"},
		nil,
	)
	swarmServiceLabel = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "service_label"),
		"Labels of the service",
		[]string{"label_name", "label_value", "service_name"},
		nil,
	)
)

type Exporter struct {
	dockerClient *docker.Client
	prometheus.Collector
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- swarmTasks
	ch <- swarmServiceReplicas
	ch <- swarmServiceRunningReplicas
	ch <- swarmServiceUpdateState
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	// get nodes

	nodes, err := e.dockerClient.ListNodes(docker.ListNodesOptions{})

	if err != nil {
		log.Printf("Error when listing nodes: %v", err)
	}

	for _, node := range nodes {
		ch <- prometheus.MustNewConstMetric(
			swarmNodes, prometheus.GaugeValue, 1, node.ID, node.Description.Hostname, string(node.Status.State), string(node.Spec.Availability),
		)
	}
	// get services

	services, err := e.dockerClient.ListServices(docker.ListServicesOptions{})

	if err != nil {
		log.Printf("Error when listing services %v", err)
	}

	for _, service := range services {
		ch <- prometheus.MustNewConstMetric(
			swarmServiceReplicas, prometheus.GaugeValue, float64(*service.Spec.Mode.Replicated.Replicas), service.Spec.Name,
		)

		if service.UpdateStatus != nil {
			ch <- prometheus.MustNewConstMetric(
				swarmServiceUpdateState, prometheus.GaugeValue, 1, service.Spec.Name, string(service.UpdateStatus.State),
			)
		}

		ch <- prometheus.MustNewConstMetric(
			swarmServiceLastUpdate, prometheus.GaugeValue, float64(service.UpdatedAt.Unix()), service.Spec.Name,
		)

		for label, value := range service.Spec.Labels {
			ch <- prometheus.MustNewConstMetric(
				swarmServiceLabel, prometheus.GaugeValue, 1, label, value, service.Spec.Name,
			)
		}

		tasks, err := e.getTasks(service)
		runningTasks := 0

		if err != nil {
			fmt.Errorf("Error when querying task %v", err)
			return
		}
		for _, task := range tasks {
			if task.Status.State == swarm.TaskStateRunning {
				runningTasks++
			}
			ch <- prometheus.MustNewConstMetric(
				swarmTasks, prometheus.GaugeValue, 1, service.Spec.Name, task.NodeID, string(task.Status.State), task.ID,
			)
		}
		ch <- prometheus.MustNewConstMetric(
			swarmServiceRunningReplicas, prometheus.GaugeValue, float64(runningTasks), service.Spec.Name,
		)

	}
}

func (e *Exporter) getNode(NodeID string) (*swarm.Node, error) {
	nodes, err := e.dockerClient.ListNodes(docker.ListNodesOptions{
		Filters: map[string][]string{
			"id": []string{NodeID},
		}},
	)
	if len(nodes) == 0 {
		return nil, fmt.Errorf("node %s doesn't seem to exist", NodeID)
	}
	if err != nil {
		return nil, err
	}
	return &nodes[0], nil

}

func (e *Exporter) getTasks(service swarm.Service) ([]swarm.Task, error) {
	tasks, err := e.dockerClient.ListTasks(docker.ListTasksOptions{
		Filters: map[string][]string{
			"service": []string{service.ID},
		}})
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

func NewExporter(dockerEndpoint string) (*Exporter, error) {
	dockerClient, err := docker.NewClient(dockerEndpoint)

	if err != nil {
		return nil, err
	}

	exporter := Exporter{
		dockerClient: dockerClient,
	}

	return &exporter, err

}

func main() {
	dockerEndpoint := flag.String("docker-endpoint", "unix:///var/run/docker.sock", "Docher endpoint.")
	metricsPath := flag.String("web.telemtry-path", "/metrics", "Path under which to expose metrics.")
	listenAddress := flag.String("web.listen-address", "0.0.0.0:9515", "Address to listen on for web interface and telemtry.")
	flag.Parse()

	exporter, err := NewExporter(*dockerEndpoint)

	if err != nil {
		log.Fatal(err)
	}

	prometheus.MustRegister(exporter)

	http.Handle(*metricsPath, prometheus.Handler())
	log.Printf("Starting swarm-exporter")
	log.Printf("Listening on %s", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
