package agent

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricPayloadSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "jscp",
			Subsystem: "agent",
			Name:      "data_readings_upload_size",
			Help:      "Data readings upload size (in bytes) sent by the jscp in-cluster agent.",
		}, []string{"organization", "cluster"})
)
