package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/sensu-community/sensu-plugin-sdk/sensu"
	"github.com/sensu/sensu-go/types"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	Debug bool
	DatabaseName string
	UserName string
}

// Setup metrics data
type metric struct {
	point string
	value string
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "sensu-go-postgres-metrics",
			Short:    "Sensu Go Postgres Metrics",
			Keyspace: "sensu.io/plugins/sensu-go-postgres-metrics/config",
		},
	}

	options = []*sensu.PluginConfigOption{
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
			Path:      "username",
			Env:       "USER_NAME",
			Argument:  "username",
			Shorthand: "u",
			Default:   "postgres",
			Usage:     "Postgres user to gather metrics",
			Value:     &plugin.UserName,
		},
	}

	metrics = []metric{}
	timestamp = time.Now().Unix()
	postgres_version float64 = 0
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
	if len(plugin.UserName) == 0 {
		return sensu.CheckStateWarning, fmt.Errorf("--username or USER_NAME environment variable is required")
	}

	return sensu.CheckStateOK, nil
}

func executeCheck(event *types.Event) (int, error) {
	if plugin.Debug {
		log.Println("executing check with: --database", plugin.DatabaseName)
		log.Println("     --database", plugin.DatabaseName)
		log.Println("     --username", plugin.UserName)
	}

  // ************************************
	// Server Details
	// ************************************

  // Hostname
	hostname, err := os.Hostname()
	if err != nil {
		return sensu.CheckStateCritical, err
	}

	// Server Version
  result, err := runQuery("show server_version;")
	if err == nil {
		postgres_version, _ = strconv.ParseFloat(strings.Split(result, " ")[0], 64)
		addMetric("postgresql.version", fmt.Sprintf("%f", postgres_version))
	}

	// ************************************
	// Replication Metrics
	// ************************************

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
		getMetric("postgresql.replication.delay", "SELECT (pg_current_wal_lsn() - confirmed_flush_lsn) AS lsn_distance FROM pg_replication_slots;")
	}

	//  - Streaming & WAL Shipping Replication Delay
	if pg_recovery_mode {
		getMetric("postgresql.replication.delay", "select (extract(epoch from (now()-pg_last_xact_replay_timestamp()))*1000)::int as replication_delay;")
	}

	if postgres_version > 0 {
		// Node Role {0 = Standalone, 1 = Master/Publisher, 2 = Slave/Subscriber}
		if pg_streaming_master || pg_logical_publisher {
			addMetric("postgresql.role", "1")
		} else if pg_streaming_slave || pg_logical_subscriber {
			addMetric("postgresql.role", "2")
		} else {
			addMetric("postgresql.role", "0")
		}

		// Replication Type {0 = None, 1 = WAL, 2 = Streaming, 3 = Logical}
		if pg_recovery_mode && !pg_streaming_slave {
			addMetric("postgresql.replication.type", "1")
		}	else if pg_streaming_master || pg_streaming_slave {
			addMetric("postgresql.replication.type", "2")
		}	else if pg_logical_publisher || pg_logical_subscriber {
			addMetric("postgresql.replication.type", "3")
		} else {
			addMetric("postgresql.replication.type", "0")
		}
	}

	// ************************************
	// Connections
	// ************************************

	getMetric("postgresql.connections." + plugin.DatabaseName + ".total", "select count(*) from pg_stat_activity;")
	getMetric("postgresql.connections." + plugin.DatabaseName + ".waiting", "select count(*) from pg_stat_activity where wait_event_type is not null;")

	// - all other states
	basequery := "select count(*) from pg_stat_activity where state = '"
	states := []string{"active", "disabled", "idle", "idle in transaction", "idle in transaction (aborted)", "fastpath function call"}
	replacer := strings.NewReplacer(" ", "_", "(", "", ")", "")
	for _, state := range states {
		getMetric("postgresql.connections." + plugin.DatabaseName + "." + replacer.Replace(state), basequery + state + "';")
	}

	// ************************************
	// Database Size
	// ************************************

	getMetric("postgresql.size." + plugin.DatabaseName, "select pg_database_size('" + plugin.DatabaseName + "');")

	// ************************************
	// Locks
	// ************************************

	getMetricsFromRows("postgresql.locks." + plugin.DatabaseName + ".", "select mode, count(mode) as count from pg_locks where database = (select oid from pg_database where datname = '" + plugin.DatabaseName + "') group by mode;")

	// ************************************
	// bgwriter
	// ************************************

	getMetricsFromColumns("postgresql.bgwriter.", "pg_stat_bgwriter", "", "", []string{"checkpoints_timed", "checkpoints_req", "checkpoint_write_time", "checkpoint_sync_time", "buffers_checkpoint", "buffers_clean", "maxwritten_clean", "buffers_backend", "buffers_backend_fsync", "buffers_alloc"})

	// ************************************
	// statsdb
	// ************************************

	getMetricsFromColumns("postgresql.statsdb." + plugin.DatabaseName + ".", "pg_stat_database", "where datname = '" + plugin.DatabaseName + "'", "", []string{"numbackends", "xact_commit", "xact_rollback", "blks_read", "blks_hit", "tup_returned", "tup_fetched", "tup_inserted", "tup_updated", "tup_deleted", "conflicts", "temp_files", "temp_bytes", "deadlocks", "blk_read_time", "blk_write_time"})

	// ************************************
	// statsio
	// ************************************

	getMetricsFromColumns("postgresql.statsio." + plugin.DatabaseName + ".", "pg_statio_user_tables", "", "sum", []string{"heap_blks_read", "heap_blks_hit", "idx_blks_read", "idx_blks_hit", "toast_blks_read", "toast_blks_hit", "tidx_blks_read", "tidx_blks_hit"})

	// ************************************
	// statstable
	// ************************************

	getMetricsFromColumns("postgresql.statstable." + plugin.DatabaseName + ".", "pg_stat_user_tables", "", "sum", []string{"seq_scan", "seq_tup_read", "idx_scan", "idx_tup_fetch", "n_tup_ins", "n_tup_upd", "n_tup_del", "n_tup_hot_upd", "n_live_tup", "n_dead_tup"})

	// ************************************
	// Print Metrics
	// ************************************

	if plugin.Debug { log.Println("printing metrics") }
	count := 0
	for _, metric := range metrics {
	    fmt.Println(hostname + "." + metric.point, metric.value, timestamp)
			count += 1
	}
	if count <= 0 {
		return sensu.CheckStateWarning, fmt.Errorf("No metric results")
	}

	// Done
	return sensu.CheckStateOK, nil
}

func addMetric(point string, value string) {
	if value == "" {
		value = "0"
	}
	metrics = append(metrics, metric{strings.ToLower(point), value})
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
			newlock := strings.Split(string(row), "|")
			addMetric(strings.ToLower(pointbase + newlock[0]), newlock[1])
		}
	}
}

func getMetric(point string, query string) {
	result, err := runQuery(query)
	if err == nil {
		addMetric(point, result)
	}
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
