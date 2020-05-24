package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitlab.tubecorporate.com/push/kafka-consumer/internal/collector"

	"gitlab.tubecorporate.com/push/kafka-consumer/pkg/converter"

	"gopkg.in/confluentinc/confluent-kafka-go.v1/kafka"

	cl "gitlab.tubecorporate.com/push/kafka-consumer/pkg/customLogger"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gitlab.tubecorporate.com/push/kafka-consumer/cmd/configStructure"
)

func isDebugMode() bool {
	if val := os.Getenv("Debug"); strings.ToLower(val) == "true" {
		return true
	}
	return false
}

func runSimpleMetricsServer(c *collector.Collector) {
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		for k, _ := range c.Tables {
			fmt.Fprintf(w, "%s - %v / %v\n", c.Tables[k].Table, c.Tables[k].GetCounter(), c.Tables[k].FlushCounter)
		}
		return
	})
	srv := &http.Server{Addr: ":8080"}
	go func() {
		cl.StdoutLog(cl.INFO, "Starting simple http server started on port :8080...", "Useful for prometheus /metrics path")
		if err := srv.ListenAndServe(); err != nil {
			cl.StdoutLog(cl.ERROR, "Server stopped", err.Error())
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func checkArguments(args []string) {
	if len(args) < 2 {
		cl.StdoutLog(cl.ERROR, "Execution error, to few arguments", "Usage: "+os.Args[0]+" [path to config file]")
		os.Exit(1)
	}
}

func loadConfig(file string) configStructure.Config {
	var config configStructure.Config
	configFile, err := os.Open(file)
	if err != nil {
		cl.StdoutLog(cl.ERROR, "Failed open config file", err.Error())
		os.Exit(1)
	}
	defer configFile.Close()
	jsonParser := json.NewDecoder(configFile)
	if err = jsonParser.Decode(&config); err != nil {
		cl.StdoutLog(cl.ERROR, "Failed parse config file (some field might be missing, or wrong json)", err.Error())
	}
	return config
}

func checkConfig(config *configStructure.Config) {
	if config.PoolTimeout <= 0 {
		config.PoolTimeout = 500
		cl.StdoutLog(cl.INFO, "Pool timeout was changed to 500ms", "Current value not correct")
	}
	for _, topic := range config.Topics {
		if topic.FlushCount <= 0 {
			topic.FlushCount = 1000
			cl.StdoutLog(cl.INFO, "Flush counter was changed to 1000 records limit", topic.Topic)
		}
		if topic.FlushIntervalSeconds <= 0 {
			topic.FlushIntervalSeconds = 300
			cl.StdoutLog(cl.INFO, "Flush interval was changed to 5 minutes", topic.Topic)
		}
		if val, _ := strconv.Atoi(topic.ClickHouseConf.WriteTimeout); val <= 0 {
			topic.ClickHouseConf.WriteTimeout = "120"
			cl.StdoutLog(cl.INFO, "ClickHouse write timeout was changed to 2 minutes", topic.Topic)
		}
	}
	cl.StdoutLog(cl.INFO, "Config parsed successfully", "")
}

func pingCHServers(config *configStructure.Config) {
	for _, topic := range config.Topics {
		response, err := http.Get("http://" + topic.ClickHouseConf.Host)
		if err != nil {
			cl.StdoutLog(cl.ERROR, "Error while ping clickHouse address", err.Error())
			os.Exit(1)
		}
		body, _ := ioutil.ReadAll(response.Body)
		_ = response.Body.Close()
		if response.Status != "400 Bad Request" || bytes.Index(body, []byte("is for clickhouse-client program")) <= 0 {
			cl.StdoutLog(cl.ERROR, "ClickHouse answer is different than expected. Waited for '400 Bad Request' tcp port", "Result - "+response.Status)
			os.Exit(1)
		}
	}
	cl.StdoutLog(cl.INFO, "ClickHouse servers pinged successfully", "")
}

func prepareCHTables(config *configStructure.Config) {
	for idx, topic := range config.Topics {
		host := topic.ClickHouseConf.Host[:strings.Index(topic.ClickHouseConf.Host, ":")]
		port := topic.ClickHouseConf.Host[strings.Index(topic.ClickHouseConf.Host, ":")+1:]
		user := topic.ClickHouseConf.User
		password := topic.ClickHouseConf.Password
		query := "describe table " + topic.ClickHouseConf.Table
		config.Topics[idx].ClickHouseConf.BaseInsertTemplateWithArguments = "clickhouse-client --host " + host + " --port " + port + " --user " + user + " --password " + password
		_, err := exec.Command("bash", "-c",
			config.Topics[idx].ClickHouseConf.BaseInsertTemplateWithArguments+" --query '"+query+"' > "+CONFIG_PATH+"/"+topic.ClickHouseConf.Table,
		).Output()
		if err != nil {
			cl.StdoutLog(cl.ERROR, "Error while getting 'describe table "+topic.ClickHouseConf.Table+"'", err.Error())
			os.Exit(1)
		}
		var chLocalStructure string
		config.Topics[idx].ColumnsMap, chLocalStructure, err = converter.ParseClickHouseTable(CONFIG_PATH + "/" + topic.ClickHouseConf.Table)
		// TODO: Игнорирование колонок
		//if len(topic.ClickHouseConf.IgnoreColumns) > 0 {
		//	for _, ignoredColumn := range topic.ClickHouseConf.IgnoreColumns {
		//		if _, exist := config.Topics[idx].ColumnsMap[ignoredColumn]; exist {
		//			delete(config.Topics[idx].ColumnsMap, ignoredColumn)
		//			cl.StdoutLog(cl.WARNING, ignoredColumn+" marked for ignore", "")
		//		}
		//	}
		//}
		if err != nil {
			cl.StdoutLog(cl.ERROR, "Error while prepare file with 'describe table "+topic.ClickHouseConf.Table+"'", err.Error())
			os.Exit(1)
		}
		config.Topics[idx].ClickHouseConf.ClickHouseLocalStructure =
			"clickhouse-local --structure=\"" + strings.Replace(chLocalStructure, "\\'", "'", -1) + "\"" +
				" --input-format='CSV'" +
				" --output-format='" + config.Topics[idx].InsertFormat + "'" +
				" --table='input'" +
				" --query='" + getQueryForClickHouseLocal(config.Topics[idx].ColumnsMap) + "'"
	}
	cl.StdoutLog(cl.INFO, "ClickHouse tables successfully parsed", "")
}

func getQueryForClickHouseLocal(tableStructure map[string]string) string {
	resultString := "SELECT "
	var columnNames []string
	for k := range tableStructure {
		columnNames = append(columnNames, k)
	}
	sort.Strings(columnNames)
	for idx, columnName := range columnNames {
		resultString += columnName
		if idx != len(columnNames)-1 {
			resultString += ", "
		}
	}
	resultString += " FROM input"
	return resultString
}

func getTopicsArray(config configStructure.Config) (resultArray []string) {
	for _, topic := range config.Topics {
		resultArray = append(resultArray, topic.Topic)
	}
	return resultArray
}

func getTableStructure(config configStructure.Config) map[string]configStructure.Topic {
	topicToTable := make(map[string]configStructure.Topic)
	for _, table := range config.Topics {
		topicToTable[table.Topic] = table
	}
	return topicToTable
}

func createConfigForConsumer(configMap map[string]string) kafka.ConfigMap {
	confMap := kafka.ConfigMap{}
	for key, val := range configMap {
		if err := confMap.SetKey(key, val); err != nil {
			cl.StderrLog(cl.ERROR, "Error set key from consumer_config", err.Error())
		}
	}
	return confMap
}

func createConsumer(configMap kafka.ConfigMap) *kafka.Consumer {
	c, loadConfigError := kafka.NewConsumer(&configMap)
	if loadConfigError != nil {
		cl.StderrLog(cl.ERROR, "Failed to create consumer", loadConfigError.Error())
		os.Exit(1)
	}
	cl.StdoutLog(cl.INFO, "Consumer created", c.String())
	return c
}

func subscribeOnTopics(consumer *kafka.Consumer, topicsArray []string) {
	if err := consumer.SubscribeTopics(topicsArray, nil); err != nil {
		cl.StderrLog(cl.ERROR, "Failed to subscribe on one or all topicToFields "+strings.Join(topicsArray, ","), err.Error())
		os.Exit(2)
	}
	cl.StdoutLog(cl.INFO, "Subscribed successfully. Waiting for messages...", strings.Join(topicsArray, ","))
}

func handleErrorMessage(msg *kafka.Message, err error) *kafka.Message {
	if err != nil || msg == nil {
		m := "null"
		e := "null"
		if msg != nil {
			m = msg.String()
		}
		if err != nil {
			e = err.Error()
		}
		if DEBUG && err != nil && err.Error() != "Local: Timed out" {
			cl.StderrLog(cl.DEBUG, m, e)
		}
		return msg
	}
	return msg
}
