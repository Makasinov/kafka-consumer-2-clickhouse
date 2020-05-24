package promMetrics

import "github.com/prometheus/client_golang/prometheus"

var (
	msgNotProcessed *prometheus.CounterVec
	msgProcessed    *prometheus.CounterVec
	dumpInsertTime  *prometheus.HistogramVec

	GetPromCounterVec   map[string]*prometheus.CounterVec
	GetPromHistogramVec map[string]*prometheus.HistogramVec
)

func init() {
	msgNotProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_consumer_msg_error",
			Help: "How many messages threw out because of unhandled error",
		}, []string{"table", "type"})
	msgProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_consumer_msg",
			Help: "How many messages got from kafka",
		}, []string{"topic", "partition"})
	dumpInsertTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dumps_time_insert",
			Help:    "Histogram of dumps inserting",
			Buckets: prometheus.LinearBuckets(0, 1, 60),
		}, []string{"table"})

	prometheus.MustRegister(
		msgNotProcessed,
		msgProcessed,
		dumpInsertTime,
	)

	GetPromCounterVec = map[string]*prometheus.CounterVec{}
	GetPromHistogramVec = map[string]*prometheus.HistogramVec{}

	GetPromCounterVec["msgNotProcessed"] = msgNotProcessed
	GetPromCounterVec["msgProcessed"] = msgProcessed
	GetPromHistogramVec["dumpInsertTime"] = dumpInsertTime
}
