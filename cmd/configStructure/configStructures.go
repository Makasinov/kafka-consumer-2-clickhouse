package configStructure

type ClickHouseConfig struct {
	User          string   `json:"user"`
	Password      string   `json:"password"`
	Host          string   `json:"host"`
	WriteTimeout  string   `json:"write_timeout"`
	Table         string   `json:"table"`
	IgnoreColumns []string `json:"ignore_columns"`
	// clickhouse-client --host host --port 9000 --user user --password pass
	BaseInsertTemplateWithArguments string
	// clickhouse-local --structure='c1 String' --input-format='CSV' --table='input' --output-format='Native' --query=''
	ClickHouseLocalStructure string
}

type Topic struct {
	ClickHouseConf       ClickHouseConfig `json:"clickhouse_config"`
	Topic                string           `json:"topic"`
	FlushCount           int              `json:"flush_count"`
	FlushIntervalSeconds int              `json:"flush_interval_seconds"`
	InsertFormat         string           `json:"insert_format"`
	ColumnsMap           map[string]string
}

type Config struct {
	Topics       []Topic           `json:"topics"`
	PoolTimeout  int               `json:"pool_timeout"`
	To           string            `json:"to"`
	ConsumerConf map[string]string `json:"consumer_config"`
}
