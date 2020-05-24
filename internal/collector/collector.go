package collector

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gitlab.tubecorporate.com/push/kafka-consumer/internal/promMetrics"

	"gitlab.tubecorporate.com/push/kafka-consumer/pkg/converter"

	cl "gitlab.tubecorporate.com/push/kafka-consumer/pkg/customLogger"

	"gitlab.tubecorporate.com/push/kafka-consumer/cmd/configStructure"
)

// FileDumper - dumps data to file system
type FileDumper struct {
	Path    string
	DumpNum int
}

type Table struct {
	columns                  map[string]string
	rows                     []string
	Table                    string
	lock                     *sync.Mutex
	counter                  int
	FlushCounter             int
	flushInterval            int
	fileDumper               *FileDumper
	baseInsertWithArguments  string
	insertFormat             string
	clickHouseLocalStructure string
}

type Collector struct {
	Tables map[string]*Table
	lock   sync.Mutex
}

//noinspection GoSnakeCaseUsage
var (
	DUMPS_PATH = "/var/log/kafka-consumer"
)

// --------------------------------------------------------------------------------------------------------------- TABLE

func newTable(t configStructure.Topic) *Table {
	return &Table{
		columns:                  t.ColumnsMap,
		rows:                     make([]string, 0, t.FlushCount),
		lock:                     &sync.Mutex{},
		counter:                  0,
		FlushCounter:             t.FlushCount,
		flushInterval:            t.FlushIntervalSeconds,
		Table:                    t.ClickHouseConf.Table,
		baseInsertWithArguments:  t.ClickHouseConf.BaseInsertTemplateWithArguments,
		insertFormat:             t.InsertFormat,
		clickHouseLocalStructure: t.ClickHouseConf.ClickHouseLocalStructure,
		fileDumper: &FileDumper{
			Path: DUMPS_PATH + "/" + t.ClickHouseConf.Table,
		},
	}
}

func (t *Table) GetCounter() int {
	t.lock.Lock()
	defer t.lock.Unlock()
	return t.counter
}

func (t *Table) append(row string) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.rows = append(t.rows, row)
	t.counter++
}

func (t *Table) Push(row string) {
	t.append(row)
	t.checkFlush()
}

func (t *Table) checkFlush() {
	t.lock.Lock()
	counter := t.counter
	t.lock.Unlock()
	if counter == t.FlushCounter {
		t.FlushBatch(false)
	}
}

func (t *Table) FlushBatch(waitInsert bool) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.counter <= 0 {
		return
	}
	cl.StdoutLog(cl.INFO, "Batch "+t.Table+" collected, dumping...", "Rows length "+strconv.Itoa(t.counter))
	pathToDump := t.fileDumper.Dump(t.Table, t.rows)
	if waitInsert {
		t.insertDump(pathToDump)
	} else {
		go t.insertDump(pathToDump)
	}
	t.rows = make([]string, 0, t.FlushCounter)
	t.counter = 0
}

func (t *Table) insertDump(path string) {
	var columns []string
	for k := range t.columns {
		columns = append(columns, k)
	}
	sort.Strings(columns)
	cmdString := "time cat " + path + " | " + t.clickHouseLocalStructure + " | " + t.baseInsertWithArguments +
		" --query='INSERT INTO " + t.Table + " (" + strings.Join(columns, ", ") + ") FORMAT " + t.insertFormat + "'"
	cmd := exec.Command("bash", "-c", cmdString)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cl.StdoutLog(cl.INFO, "Start insert", path)
	start := time.Now()
	if err := cmd.Run(); err != nil {
		/*
			TODO: Манажить не вставленными пачками
			TODO: Как минимум перевставлять их если сетка моргнула во время процесса
			TODO: Или ещё че
		*/
		// TODO: А и ещё прикрутить слак алёрт
		// Хуле так дохуя делать то ебана :C
		fmt.Println("Insert error. DEBUG INFO:")
		fmt.Println(err.Error())
		fmt.Println(err.(*exec.ExitError).ExitCode())
		fmt.Println(stderr.String())
		fmt.Println(cmdString)
		return
	}
	promMetrics.GetPromHistogramVec["dumpInsertTime"].
		WithLabelValues(t.Table).
		Observe(float64(time.Since(start).Seconds()))
	if _, err := exec.Command("bash", "-c", "rm -f "+path).Output(); err != nil {
		cl.StdoutLog(cl.WARNING, "Error remove batch after insert in"+time.Since(start).String()+" ("+path+")", err.Error())
		return
	}
	cl.StdoutLog(cl.INFO, "Inserted in "+time.Since(start).String()+" and removed", path)
}

// ----------------------------------------------------------------------------------------------------------- COLLECTOR

func NewCollector() *Collector {
	return &Collector{
		Tables: map[string]*Table{},
		lock:   sync.Mutex{},
	}
}

func (c *Collector) FlushAll() {
	for _, table := range c.Tables {
		table.FlushBatch(true)
	}
}

func (c *Collector) Push(b []byte, t configStructure.Topic) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	row, err := converter.GetParsedMessage(b, t.ColumnsMap)
	if err != nil {
		return err
	}

	if table, founded := c.Tables[t.ClickHouseConf.Table]; founded == true {
		table.Push(row)
	} else {
		c.Tables[t.ClickHouseConf.Table] = newTable(t)
		c.Tables[t.ClickHouseConf.Table].Push(row)
	}
	return nil
}

// --------------------------------------------------------------------------------------------------------- FILE DUMPER

// Dump - dumps data to files
func (d *FileDumper) Dump(tableName string, data []string) string {
	if _, err := os.Stat(d.Path); os.IsNotExist(err) {
		_ = os.Mkdir(d.Path, os.ModePerm)
	}
	d.DumpNum++
	sec := time.Now().Unix()
	dumpName := strconv.Itoa(int(sec)) + "." + tableName + strconv.Itoa(d.DumpNum) + ".csv"
	pathToDump := path.Join(d.Path, dumpName)
	rowsLength := strconv.Itoa(len(data))
	_ = os.MkdirAll(d.Path, os.ModePerm)
	err := ioutil.WriteFile(pathToDump, []byte(strings.Join(data, "\n")), os.ModePerm)
	if err == nil {
		cl.StdoutLog(cl.INFO, dumpName+" of "+rowsLength+" rows saved "+tableName, pathToDump)
	} else {
		cl.StderrLog(cl.WARNING, dumpName+" of "+rowsLength+" rows NOT SAVED! "+tableName, err.Error())
	}
	return pathToDump
}
