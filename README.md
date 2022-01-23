[![Sensu Bonsai Asset](https://img.shields.io/badge/Bonsai-Download%20Me-brightgreen.svg?colorB=89C967&logo=sensu)](https://bonsai.sensu.io/assets/scottcupit/sensu-go-postgres)
![Go Test](https://github.com/scottcupit/sensu-go-postgres/workflows/Go%20Test/badge.svg)
![goreleaser](https://github.com/scottcupit/sensu-go-postgres/workflows/goreleaser/badge.svg)

# Sensu Go PostgreSQL Check and Metrics

## Table of Contents
- [Overview](#overview)
- [Known issues](#known-issues)
- [Usage examples](#usage-examples)
  - [Setup Postgres User](#setup-postgres-user)
  - [Examples](#examples)
- [Configuration](#configuration)
  - [Asset registration](#asset-registration)
  - [Metric definition](#metric-definition)
  - [Check definition](#check-definition)
- [Installation from source](#installation-from-source)
- [Additional notes](#additional-notes)
- [Contributing](#contributing)

## Overview

The sensu-go-postgres is a sensu check that collects PostgreSQL metrics.

Metrics are output in Graphite form.

Supports PostgreSQL version >= 10.

This is the first release, mainly testing integration into my environment for now.

## Known issues

* This is my first time writing in Go and contributing to Sensu, unexpected issues may occur, please let me know
* Check uses system installed `psql` command, in future will migrate to go library such as pq
* Check does not take connection paremeters for host, port, etc., will add when migrated to library
* Output is not sorted, future release
* Replication delay metric only works for WAL shipping, streaming, and logical replication.  WAL shipping and streaming replication delay is calculated on the slave server, logical on the publisher
* Logical replication only works with one subscriber

## Usage examples

### Setup Postgres User

#### pg_hba.conf

Add this entry to your pg_hba.conf file
```
# TYPE  DATABASE        USER            ADDRESS                 METHOD
local   postgres        sensu                                   peer
```

#### Role

Add this role to your PostgreSQL server

```
CREATE ROLE sensu NOSUPERUSER INHERIT NOCREATEDB NOCREATEROLE NOREPLICATION LOGIN VALID UNTIL 'infinity';
GRANT CONNECT ON DATABASE postgres TO sensu;
GRANT pg_monitor TO sensu;
```

### Examples

#### Print all metrics

##### Command
```
sensu-go-postgres --database postgres --username sensu
```

##### Output
```
HOSTNAME.postgresql.version 12.8 1642027852
HOSTNAME.postgresql.replication.role 0 1642027852
HOSTNAME.postgresql.replication.type 0 1642027852
HOSTNAME.postgresql.connections.postgres.total 7 1642027852
HOSTNAME.postgresql.connections.postgres.waiting 5 1642027852
HOSTNAME.postgresql.connections.postgres.active 2 1642027852
HOSTNAME.postgresql.connections.postgres.disabled 0 1642027852
HOSTNAME.postgresql.connections.postgres.idle 0 1642027852
HOSTNAME.postgresql.connections.postgres.idle_in_transaction 0 1642027852
HOSTNAME.postgresql.connections.postgres.idle_in_transaction_aborted 0 1642027852
HOSTNAME.postgresql.connections.postgres.fastpath_function_call 0 1642027852
HOSTNAME.postgresql.size.postgres 8209263 1642027852
HOSTNAME.postgresql.locks.postgres.accesssharelock 1 1642027852
HOSTNAME.postgresql.bgwriter.checkpoints_timed 42008 1642027852
HOSTNAME.postgresql.bgwriter.checkpoints_req 327 1642027852
HOSTNAME.postgresql.bgwriter.checkpoint_write_time 8900302 1642027852
HOSTNAME.postgresql.bgwriter.checkpoint_sync_time 38969 1642027852
HOSTNAME.postgresql.bgwriter.buffers_checkpoint 2429237 1642027852
HOSTNAME.postgresql.bgwriter.buffers_clean 230116 1642027852
HOSTNAME.postgresql.bgwriter.maxwritten_clean 1955 1642027852
HOSTNAME.postgresql.bgwriter.buffers_backend 2683706 1642027852
HOSTNAME.postgresql.bgwriter.buffers_backend_fsync 0 1642027852
HOSTNAME.postgresql.bgwriter.buffers_alloc 65433213 1642027852
HOSTNAME.postgresql.statsdb.postgres.numbackends 1 1642027852
HOSTNAME.postgresql.statsdb.postgres.xact_commit 450264 1642027852
HOSTNAME.postgresql.statsdb.postgres.xact_rollback 1 1642027852
HOSTNAME.postgresql.statsdb.postgres.blks_read 1638 1642027852
HOSTNAME.postgresql.statsdb.postgres.blks_hit 19555338 1642027852
HOSTNAME.postgresql.statsdb.postgres.tup_returned 261429620 1642027852
HOSTNAME.postgresql.statsdb.postgres.tup_fetched 3044430 1642027852
HOSTNAME.postgresql.statsdb.postgres.tup_inserted 0 1642027852
HOSTNAME.postgresql.statsdb.postgres.tup_updated 0 1642027852
HOSTNAME.postgresql.statsdb.postgres.tup_deleted 0 1642027852
HOSTNAME.postgresql.statsdb.postgres.conflicts 0 1642027852
HOSTNAME.postgresql.statsdb.postgres.temp_files 0 1642027852
HOSTNAME.postgresql.statsdb.postgres.temp_bytes 0 1642027852
HOSTNAME.postgresql.statsdb.postgres.deadlocks 0 1642027852
HOSTNAME.postgresql.statsdb.postgres.blk_read_time 0 1642027852
HOSTNAME.postgresql.statsdb.postgres.blk_write_time 0 1642027852
HOSTNAME.postgresql.statsio.postgres.heap_blks_read 0 1642027852
HOSTNAME.postgresql.statsio.postgres.heap_blks_hit 0 1642027852
HOSTNAME.postgresql.statsio.postgres.idx_blks_read 0 1642027852
HOSTNAME.postgresql.statsio.postgres.idx_blks_hit 0 1642027852
HOSTNAME.postgresql.statsio.postgres.toast_blks_read 0 1642027852
HOSTNAME.postgresql.statsio.postgres.toast_blks_hit 0 1642027852
HOSTNAME.postgresql.statsio.postgres.tidx_blks_read 0 1642027852
HOSTNAME.postgresql.statsio.postgres.tidx_blks_hit 0 1642027852
HOSTNAME.postgresql.statstable.postgres.seq_scan 0 1642027852
HOSTNAME.postgresql.statstable.postgres.seq_tup_read 0 1642027852
HOSTNAME.postgresql.statstable.postgres.idx_scan 0 1642027852
HOSTNAME.postgresql.statstable.postgres.idx_tup_fetch 0 1642027852
HOSTNAME.postgresql.statstable.postgres.n_tup_ins 0 1642027852
HOSTNAME.postgresql.statstable.postgres.n_tup_upd 0 1642027852
HOSTNAME.postgresql.statstable.postgres.n_tup_del 0 1642027852
HOSTNAME.postgresql.statstable.postgres.n_tup_hot_upd 0 1642027852
HOSTNAME.postgresql.statstable.postgres.n_live_tup 0 1642027852
HOSTNAME.postgresql.statstable.postgres.n_dead_tup 0 1642027852
```

#### Print specific metrics

##### Command
```
sensu-go-postgres --database postgres --username sensu --metrics connections,locks
```

##### Output
```
HOSTNAME.postgresql.connections.postgres.total 7 1642027852
HOSTNAME.postgresql.connections.postgres.waiting 5 1642027852
HOSTNAME.postgresql.connections.postgres.active 2 1642027852
HOSTNAME.postgresql.connections.postgres.disabled 0 1642027852
HOSTNAME.postgresql.connections.postgres.idle 0 1642027852
HOSTNAME.postgresql.connections.postgres.idle_in_transaction 0 1642027852
HOSTNAME.postgresql.connections.postgres.idle_in_transaction_aborted 0 1642027852
HOSTNAME.postgresql.connections.postgres.fastpath_function_call 0 1642027852
HOSTNAME.postgresql.locks.postgres.accesssharelock 1 1642027852
```

#### Check specific metric

##### Command
```
sensu-go-postgres --database postgres --username sensu --check connections.postgres.total --warning 100 --critical 200
```

##### Output
```
OK: connections.postgres.total = 7.000000
```

## Configuration

### Asset registration

[Sensu Assets][10] are the best way to make use of this plugin. If you're not using an asset, please
consider doing so! If you're using sensuctl 5.13 with Sensu Backend 5.13 or later, you can use the
following command to add the asset:

```
sensuctl asset add scottcupit/sensu-go-postgres
```

If you're using an earlier version of sensuctl, you can find the asset on the [Bonsai Asset Index][https://bonsai.sensu.io/assets/scottcupit/sensu-go-postgres].

### Metric definition

```yml
---
type: CheckConfig
api_version: core/v2
metadata:
  name: metrics-postgres
  namespace: default
spec:
  command: sensu-go-postgres --database {{index .labels "postgres_database" | default "postgres"}} --username {{.labels.postgres_username | default "sensu"}}
  interval: 30
  output_metric_format: graphite_plaintext
  output_metric_handlers:
  - graphite
  publish: true
  subscriptions:
  - postgres
  runtime_assets:
  - scottcupit/sensu-go-postgres
```

### Check definition

```yml
---
type: CheckConfig
api_version: core/v2
metadata:
  name: check-postgres-connections
  namespace: default
spec:
  command: sensu-go-postgres --database {{index .labels "postgres_database" | default "postgres"}} --username {{.labels.postgres_username | default "sensu"}} --check connections.{{.labels.postgres_database | default "postgres"}}.total --warning 500 --critical 1000
  interval: 60
  output_metric_format: graphite_plaintext
  output_metric_handlers:
  - graphite
  publish: true
  subscriptions:
  - postgres
  runtime_assets:
  - scottcupit/sensu-go-postgres
```

## Installation from source

The preferred way of installing and deploying this plugin is to use it as an Asset. If you would
like to compile and install the plugin from source or contribute to it, download the latest version
or create an executable script from this source.

From the local path of the sensu-go-postgres repository:

```
go build
```

## Additional notes

### Replication statistics

#### Role
```
0 = Standalone
1 = Master/Publisher
2 = Slave/Subscriber
```

#### Replication Type
```
0 = None
1 = WAL
2 = Streaming
3 = Logical
```

## Contributing

For more information about contributing to this plugin, see [Contributing][1].

[1]: https://github.com/sensu/sensu-go/blob/master/CONTRIBUTING.md
[2]: https://github.com/sensu-community/sensu-plugin-sdk
[3]: https://github.com/sensu-plugins/community/blob/master/PLUGIN_STYLEGUIDE.md
[4]: https://github.com/sensu-community/check-plugin-template/blob/master/.github/workflows/release.yml
[5]: https://github.com/sensu-community/check-plugin-template/actions
[6]: https://docs.sensu.io/sensu-go/latest/reference/checks/
[7]: https://github.com/sensu-community/check-plugin-template/blob/master/main.go
[8]: https://bonsai.sensu.io/
[9]: https://github.com/sensu-community/sensu-plugin-tool
[10]: https://docs.sensu.io/sensu-go/latest/reference/assets/
