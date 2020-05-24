package main

import "C"
import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/golang/snappy"

	"gitlab.tubecorporate.com/push/kafka-consumer/internal/promMetrics"

	"gitlab.tubecorporate.com/push/kafka-consumer/internal/collector"

	cl "gitlab.tubecorporate.com/push/kafka-consumer/pkg/customLogger"
)

//noinspection GoSnakeCaseUsage
var (
	CONFIG_PATH = "/etc/kafka-consumer/conf.d"
	DEBUG       = isDebugMode()
)

func main() {
	checkArguments(os.Args)
	config := loadConfig(os.Args[1])
	checkConfig(&config)

	// Ping ClickHouse servers for activity
	pingCHServers(&config)

	// Prepare ClickHouse "describe table <TABLE>" structure for converter service
	prepareCHTables(&config)

	// Create consumer with passed configuration
	kafkaConfigMap := createConfigForConsumer(config.ConsumerConf)
	consumer := createConsumer(kafkaConfigMap)

	batchCollector := collector.NewCollector()

	// Run simple server with prometheus metrics
	go runSimpleMetricsServer(batchCollector)

	subscribeOnTopics(consumer, getTopicsArray(config))

	tableByName := getTableStructure(config)

	// Listen for SIGTERM interruption
	exitChan := make(chan os.Signal, 1)
	signal.Notify(exitChan, syscall.SIGTERM)
	signal.Notify(exitChan, syscall.SIGINT)
	signal.Notify(exitChan, syscall.SIGKILL)

	firstMsg := true
	for {
		select {
		case sig := <-exitChan:
			cl.StdoutLog(cl.ERROR, "Termination signal called", sig.String())
			if err := consumer.Close(); err != nil {
				cl.StderrLog(cl.INFO, "Error close consumer", err.Error())
			}
			cl.StderrLog(cl.INFO, "Consumer closed successfully", "")
			batchCollector.FlushAll()
			cl.StderrLog(cl.INFO, "All ClickHouse tables with data were flushed ", "")
			cl.StderrLog(cl.INFO, "Have a good day!", "")
			os.Exit(1)

		// Receive message from kafka
		default:
			msg := handleErrorMessage(
				consumer.ReadMessage(time.Duration(config.PoolTimeout) + time.Millisecond),
			)
			if msg == nil {
				continue
			}
			decoded, err := snappy.Decode(nil, msg.Value)
			if firstMsg {
				cl.StdoutLog(cl.INFO, "Received first message", string(decoded))
				firstMsg = false
			}
			if DEBUG {
				fmt.Println(string(decoded))
			}
			if err != nil {
				if DEBUG {
					cl.StderrLog(cl.WARNING, "Error snappy decode", err.Error())
				}
				promMetrics.GetPromCounterVec["msgNotProcessed"].WithLabelValues(
					tableByName[*msg.TopicPartition.Topic].Topic,
					err.Error(),
				).Inc()
			}
			if err := batchCollector.Push(decoded, tableByName[*msg.TopicPartition.Topic]); err != nil {
				if DEBUG {
					cl.StderrLog(cl.WARNING, "Error push message to collector. "+err.Error(), string(msg.Value))
				}
				promMetrics.GetPromCounterVec["msgNotProcessed"].WithLabelValues(
					tableByName[*msg.TopicPartition.Topic].Topic,
					err.Error(),
				).Inc()
			} else {
				promMetrics.GetPromCounterVec["msgProcessed"].WithLabelValues(
					tableByName[*msg.TopicPartition.Topic].Topic,
					strconv.Itoa(int(msg.TopicPartition.Partition)),
				).Inc()
			}
		}
	}
}
