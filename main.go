package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/sensu-community/sensu-plugin-sdk/sensu"
	"github.com/sensu/sensu-go/types"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	Check string
	Critical float64
	Debug bool
	DatabaseName string
	Metrics []string
	UserName string
	Warning float64
}

// Setup check functions & metric data
type check struct{}

type metric struct {
	point string
	value string
}

var (
		points = []string{"version", "bgwriter", "connections", "locks", "replication", "size", "statsdb", "statsio", "statstable"}
		metrics = []metric{}
		timestamp = time.Now().Unix()
		postgres_version float64 = 0
		hostname = getHostName()
)

// Setup plugin
var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "sensu-go-postgres",
			Short:    "Sensu Go PostgreSQL Check and Metrics",
			Keyspace: "sensu.io/plugins/sensu-go-postgres/config",
		},
	}

	options = []*sensu.PluginConfigOption{
		&sensu.PluginConfigOption{
			Path:      "check",
			Env:       "Check",
			Argument:  "check",
			Shorthand: "k",
			Default:   "",
			Usage:     "Run check for a specific metric",
			Value:     &plugin.Check,
		},
		&sensu.PluginConfigOption{
			Path:      "critical",
			Env:       "Critical",
			Argument:  "critical",
			Shorthand: "c",
			Default:   95.0,
			Usage:     "Critical threshold for specific metric check",
			Value:     &plugin.Critical,
		},
		&sensu.PluginConfigOption{
			Path:      "debug",
			Env:       "DEBUG",
			Argument:  "debug",
			Shorthand: "l",
			Default:   false,
			Usage:     "Print debug log messages",
			Value:     &plugin.Debug,
		},
		&sensu.PluginConfigOption{
			Path:      "database",
			Env:       "DATABASE_NAME",
			Argument:  "database",
			Shorthand: "d",
			Default:   "sensu",
			Usage:     "Database to collect metrics",
			Value:     &plugin.DatabaseName,
		},
		&sensu.PluginConfigOption{
			Path:      "metrics",
			Env:       "METRICS",
			Argument:  "metrics",
			Shorthand: "m",
			Default:   points,
			Usage:     "Metrics to check",
			Value:     &plugin.Metrics,
		},
		&sensu.PluginConfigOption{
			Path:      "username",
			Env:       "USER_NAME",
			Argument:  "username",
			Shorthand: "u",
			Default:   "sensu",
			Usage:     "Postgres user to gather metrics",
			Value:     &plugin.UserName,
		},
		&sensu.PluginConfigOption{
			Path:      "warning",
			Env:       "Warning",
			Argument:  "warning",
			Shorthand: "w",
			Default:   85.0,
			Usage:     "Warning threshold for specific metric check",
			Value:     &plugin.Warning,
		},
	}
)

func main() {
	useStdin := false
	fi, err := os.Stdin.Stat()
	if err != nil {
		fmt.Printf("Error check stdin: %v\n", err)
		panic(err)
	}
	//Check the Mode bitmask for Named Pipe to indicate stdin is connected
	if fi.Mode()&os.ModeNamedPipe != 0 {
		log.Println("using stdin")
		useStdin = true
	}

	check := sensu.NewGoCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, useStdin)
	check.Execute()
}

func checkArgs(event *types.Event) (int, error) {
	if len(plugin.DatabaseName) == 0 {
		return sensu.CheckStateWarning, fmt.Errorf("--database or DATABASE_NAME environment variable is required")
	}
	if len(plugin.Check) > 0 {
		plugin.Metrics = []string{}
		if plugin.Critical <= plugin.Warning {
			return sensu.CheckStateWarning, fmt.Errorf("--critical threshold must be larger than --warning threshold")
		}
		if ! arrayContains(strings.Split(plugin.Check, ".")[0], points) {
			return sensu.CheckStateWarning, fmt.Errorf("--check is not supported: %s", plugin.Check)
		}
	}
	if len(plugin.Metrics) > 0 {
		for _, metric := range plugin.Metrics {
			if ! arrayContains(metric, points) {
				return sensu.CheckStateWarning, fmt.Errorf("--metrics not supported: %s", metric)
			}
		}
	}
	if len(plugin.UserName) == 0 {
		return sensu.CheckStateWarning, fmt.Errorf("--username or USER_NAME environment variable is required")
	}

	return sensu.CheckStateOK, nil
}

func executeCheck(event *types.Event) (int, error) {
	if plugin.Debug {
		log.Println("executing sensu-go-postgres with:")
		log.Println("     --database", plugin.DatabaseName)
		log.Println("     --username", plugin.UserName)
	}

	checks := reflect.ValueOf(check{})

	// Run check on a single metric
	if len(plugin.Check) > 0 {
		if plugin.Debug {
			log.Println("     --check", plugin.Check)
			log.Println("     --warning", fmt.Sprintf("%f", plugin.Warning))
			log.Println("     --critical", fmt.Sprintf("%f", plugin.Critical))
		}

		// Use reflection to dynamicaly call the requested check and get metrics
		check := strings.Split(plugin.Check, ".")[0]
		function := checks.MethodByName(strings.Title(check))
		if function.Kind() != reflect.Func {
			return sensu.CheckStateWarning, fmt.Errorf("point not supported: %s", check)
		}
		if plugin.Debug { log.Println("check:", check) }
		function.Call(nil)

		// Extract desired metric
		check_found := false
		check_current := 0.0
		for _, metric := range metrics {
			if metric.point == plugin.Check {
				check_found = true
				check_current, _ = strconv.ParseFloat(metric.value, 64)
			}
		}

		// Determine check state
		if check_found {
			switch {
			case check_current >= plugin.Critical:
				fmt.Printf("CRITICAL: %s = %f\n", plugin.Check, check_current)
				return sensu.CheckStateCritical, nil
			case check_current >= plugin.Warning:
				fmt.Printf("WARNING: %s = %f\n", plugin.Check, check_current)
				return sensu.CheckStateWarning, nil
			default:
				fmt.Printf("OK: %s = %f\n", plugin.Check, check_current)
				return sensu.CheckStateOK, nil
			}
		}

		_ = printMetrics("")
 		return sensu.CheckStateWarning, fmt.Errorf("point not found: %s", check)
	}

	// Run multiple checks/metrics
	if plugin.Debug { log.Println("     --metrics:", plugin.Metrics) }

	// Use reflection to dynamicaly call the requested checks/metrics
	for _, point := range plugin.Metrics {
		function := checks.MethodByName(strings.Title(point))
		if function.Kind() != reflect.Func {
			return sensu.CheckStateWarning, fmt.Errorf("point not supported: %s", point)
		}
		if plugin.Debug { log.Println("check:", point) }
		function.Call(nil)
	}

	metric_count := printMetrics(hostname + ".postgresql.")
	if metric_count == 0 {
		return sensu.CheckStateWarning, fmt.Errorf("No metrics found")
	}

	return sensu.CheckStateOK, nil
}

// ************************************
// Checks/Metrics
// ************************************

// bgwriter
func (check) Bgwriter() {
	getMetricsFromColumns("bgwriter.", "pg_stat_bgwriter", "", "", []string{"checkpoints_timed", "checkpoints_req", "checkpoint_write_time", "checkpoint_sync_time", "buffers_checkpoint", "buffers_clean", "maxwritten_clean", "buffers_backend", "buffers_backend_fsync", "buffers_alloc"})
}

// Connections
func (check) Connections() {
	getMetric("connections." + plugin.DatabaseName + ".total", "select count(*) from pg_stat_activity;")
	getMetric("connections." + plugin.DatabaseName + ".waiting", "select count(*) from pg_stat_activity where wait_event_type is not null;")

	// - all other states
	basequery := "select count(*) from pg_stat_activity where state = '"
	states := []string{"active", "disabled", "idle", "idle in transaction", "idle in transaction (aborted)", "fastpath function call"}
	replacer := strings.NewReplacer(" ", "_", "(", "", ")", "")
	for _, state := range states {
		getMetric("connections." + plugin.DatabaseName + "." + replacer.Replace(state), basequery + state + "';")
	}
}

// Locks
func (check) Locks() {
	// Lock modes
	getMetricsFromRows("locks." + plugin.DatabaseName + ".", "select mode, count(mode) as count from pg_locks where database = (select oid from pg_database where datname = '" + plugin.DatabaseName + "') group by mode;")

	// Total locks
	count := 0.0
	for _, metric := range metrics {
		point := strings.Split(metric.point, ".")
		if len(point) > 1 {
			if point[0] + "." + point[1] == "locks." + plugin.DatabaseName {
				value, _ := strconv.ParseFloat(metric.value, 64)
				count += value
			}
		}
	}
	addMetric("locks." + plugin.DatabaseName + ".total", fmt.Sprintf("%f", count))
}

// Replication
func (check) Replication() {
	pg_recovery_mode := runValidateQuery("select pg_is_in_recovery();", "t")
	pg_logical_publisher := runValidateQuery("select slot_type from pg_replication_slots where slot_type='logical' and active='t';", "logical")

	pg_streaming_master := false
	if !pg_logical_publisher {
		pg_streaming_master = runValidateQuery("select state from pg_stat_replication WHERE state='streaming';", "streaming")
	}

	pg_streaming_slave := runValidateQuery("select status from pg_stat_wal_receiver WHERE status='streaming';", "streaming")

	pg_logical_subscriber := false
	if !pg_streaming_slave {
		pg_logical_subscriber = runValidateQuery("select wait_event from pg_stat_activity WHERE wait_event='LogicalApplyMain';", "LogicalApplyMain")
	}

	//  - Logical Replication Delay
	if pg_logical_publisher {
		getMetric("replication.delay", "SELECT (pg_current_wal_lsn() - confirmed_flush_lsn) AS lsn_distance FROM pg_replication_slots;")
	}

	//  - Streaming & WAL Shipping Replication Delay
	if pg_recovery_mode {
		getMetric("replication.delay", "select (extract(epoch from (now()-pg_last_xact_replay_timestamp()))*1000)::bigint as replication_delay;")
	}

	if postgres_version == 0 { check{}.Version() }
	if postgres_version > 0 {
		// Node Role {0 = Standalone, 1 = Master/Publisher, 2 = Slave/Subscriber}
		if plugin.Debug { log.Println("determine role") }
		if pg_streaming_master || pg_logical_publisher {
			addMetric("replication.role", "1")
		} else if pg_streaming_slave || pg_logical_subscriber {
			addMetric("replication.role", "2")
		} else {
			addMetric("replication.role", "0")
		}

		// Replication Type {0 = None, 1 = WAL, 2 = Streaming, 3 = Logical}
		if plugin.Debug { log.Println("determine replication type") }
		if pg_recovery_mode && !pg_streaming_slave {
			addMetric("replication.type", "1")
		}	else if pg_streaming_master || pg_streaming_slave {
			addMetric("replication.type", "2")
		}	else if pg_logical_publisher || pg_logical_subscriber {
			addMetric("replication.type", "3")
		} else {
			addMetric("replication.type", "0")
		}
	}
}

// Database Size
func (check) Size() {
	getMetric("size." + plugin.DatabaseName, "select pg_database_size('" + plugin.DatabaseName + "');")
}

// statsdb
func (check) Statsdb() {
	getMetricsFromColumns("statsdb." + plugin.DatabaseName + ".", "pg_stat_database", "where datname = '" + plugin.DatabaseName + "'", "", []string{"numbackends", "xact_commit", "xact_rollback", "blks_read", "blks_hit", "tup_returned", "tup_fetched", "tup_inserted", "tup_updated", "tup_deleted", "conflicts", "temp_files", "temp_bytes", "deadlocks", "blk_read_time", "blk_write_time"})
}

// statsio
func (check) Statsio() {
	getMetricsFromColumns("statsio." + plugin.DatabaseName + ".", "pg_statio_user_tables", "", "sum", []string{"heap_blks_read", "heap_blks_hit", "idx_blks_read", "idx_blks_hit", "toast_blks_read", "toast_blks_hit", "tidx_blks_read", "tidx_blks_hit"})
}

// statstable
func (check) Statstable() {
	getMetricsFromColumns("statstable." + plugin.DatabaseName + ".", "pg_stat_user_tables", "", "sum", []string{"seq_scan", "seq_tup_read", "idx_scan", "idx_tup_fetch", "n_tup_ins", "n_tup_upd", "n_tup_del", "n_tup_hot_upd", "n_live_tup", "n_dead_tup"})
}

// Server Version
func (check) Version() {
	result, err := runQuery("show server_version;")
	if err == nil {
		postgres_version, _ = strconv.ParseFloat(strings.Split(result, " ")[0], 64)
		addMetric("version", fmt.Sprintf("%f", postgres_version))
	}
}

// ************************************
// Support Functions
// ************************************

func addMetric(point string, value string) {
	if value == "" {
		value = "0"
	}
	metrics = append(metrics, metric{strings.ToLower(point), value})
}

func arrayContains(criteria string, search []string) (bool) {
	for _, value := range search {
		if value == criteria {
			return true
		}
	}

	return false
}

func getHostName() (string) {
	if plugin.Debug { log.Println("get hostname") }

	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}

	return strings.Replace(hostname, ".", "-", -1)
}

func getMetricsFromColumns(pointbase string, table string, where string, math string, points []string) {
	columns := ""
	for i, point := range points {
		if math != "" {
			columns += math + "(" + point + ")"
		} else {
			columns += point
		}
		if i < len(points) - 1 {
			columns += ", "
		}
	}
	result, err := runQuery("select " + columns + " from " + table + " " + where + ";")
	if err == nil {
		values := strings.Split(result, "|")
		for i, point := range points {
			addMetric(pointbase + point, values[i])
		}
	}
}

func getMetricsFromRows(pointbase string, query string) {
	result, err := runQuery(query)
	if err == nil {
		for _, row := range strings.Split(result, "\n") {
			newrow := strings.Split(string(row), "|")
			addMetric(strings.ToLower(pointbase + newrow[0]), newrow[1])
		}
	}
}

func getMetric(point string, query string) {
	result, err := runQuery(query)
	if err == nil {
		addMetric(point, result)
	}
}

func printMetrics(basestring string) (int) {
	if plugin.Debug { log.Println("printing metrics") }
	count := 0
	for _, metric := range metrics {
	    fmt.Println(basestring + metric.point, metric.value, timestamp)
			count += 1
	}

	return count
}

func runQuery(query string) (string, error) {
	if plugin.Debug { log.Println("running query:", query) }

	cmd := exec.Command("psql", plugin.DatabaseName, "-U", plugin.UserName, "-t", "-A", "-c", query)
	out, err := cmd.Output()
	result := strings.TrimSuffix(string(out), "\n")

	if plugin.Debug { log.Println(" - result:", result) }
	return result, err
}

func runValidateQuery(query string, response string) (bool) {
	result, err := runQuery(query)
	if err == nil {
		if result == response {
			if plugin.Debug { log.Println(" - validate:", response, "=", "true") }
			return true
		}
	}

	if plugin.Debug { log.Println(" - validate:", response, "=", "false") }
	return false
}
