# PromHouse

[![Build Status](https://travis-ci.org/hmrc/Promhouse.svg?branch=master)](https://travis-ci.org/hmrc/Promhouse)
[![codecov](https://codecov.io/gh/hmrc/Promhouse/branch/master/graph/badge.svg)](https://codecov.io/gh/hmrc/Promhouse)
[![Go Report Card](https://goreportcard.com/badge/github.com/hmrc/Promhouse)](https://goreportcard.com/report/github.com/hmrc/Promhouse)
[![CLA assistant](https://cla-assistant.percona.com/readme/badge/hmrc/Promhouse)](https://cla-assistant.percona.com/hmrc/Promhouse)

PromHouse is a long-term remote storage with built-in clustering and downsampling for 2.x on top of
[ClickHouse](https://clickhouse.yandex). Or, rather, it will be someday.
Feel free to ~~like, share, retweet,~~ star and watch it, but **do not use it in production** yet.

## Generating the binaries
If you just want the binaries to play around with, run
```
bash make build
```

## Setting up the dev environment
This is a go project. You will need to:
1. Install go
2. Setup your GOPATH. I recomend using the default (`${HOME}/go`). Add to your `.bashrc` or `.bash_profile`

```bash
export GOPATH=${HOME}/go
```
3. Add the go binary directory to your path. Add to your `.bashrc` or `.bash_profile`
```bash
export PATH="${GOPATH}/bin:$PATH"
```
4. Checkout this repo under `${GOPATH}/src/github.com/hmrc/Promhouse`. This is important for the dependencies to be picked up correctly. 
5. You are good to go!

## Compiling
1. make init
2. make protos
3. make install

## Running the tests
You need the test environment up to run the tests
1. In a terminal run:
```
make up
```
2. In a different terminal run:
```
make run
```
3. In yet another terminal run:
```
make tests
```
4. Fuzz tests:
```
make gofuzz
```

## Runing memory and cpu profiling
### Prerequisites
You will need to install graphviz:
```bash
apt-get install graphviz
```
Or if you are in mac:
```bash
brew install graphviz
```

### Generate the profiling data
Run promhouse with either cpu or mem profiling enabled by running it with:
```bash
make run-memprofile
```
Or
```bash
make run-cpuprofile
```
Run any tests you want to profile (for example, generating some load), and stop promhouse. You will see where the pprof file is created in an output like this:
```
promhouse --log.level=info --profile.mem
stdlog: profile: memory profiling enabled (rate 4096), /tmp/profile371198043/mem.pprof
```
The profiling needs some data. If you do not run it for long enough (allow it at least 30 secs or so), you won't get enough data and the profiling won't work.

NOTE: the promhouse binary needs to be in your path. That would probably look like this:
```bash
export PATH="${GOPATH}/bin:$PATH"
```

### Graph the gathered data
You can now use [pprof](https://github.com/google/pprof) to read the data. The quickest way I found is to generate a pdf:
```bash
go tool pprof --pdf ${GOPATH}/bin/promhouse /tmp/profile382238388/mem.pprof > profilegraph.pdf
```

## Manually testing load
If you want to see how Prometheus and PromHouse behave under load. You don't have any dependencies (not go or anything else) apart from docker and compose.
1. Build the binaries and start up up the test environment in a terminal. To do that, run:
```bash
make up
```
or if you are in mac:
```bash
make up-mac
```
This will start all the test environment (grafana, prometheus...) and promhouse.
2. Generate some load. In another terminal run:
```bash
make generate-load
```
This will start up a docker container with Avalanch, and will generate metrics (default settings).

### Graph what is going on
You can go to the Prometheus console to see what is going on, in `http://127.0.0.1:9090/graph`.
Under graph, you can add serveral graphs and see what is happening
Useful queries to graph are:
1. `process_resident_memory_bytes` : to see how much memory each of the components is using. PromHouse and Prometheus will most likely be the highest consumers
NOTE: you have limits defined in the compose file. They are set to a maximum of 2.5GB for each one. If it is too much or too little, you can modify them.
2. `scrape_samples_scraped`: to visualize how many samples have been processed.
3. `prometheus_remote_storage_queue_length`: to see the PromHouse queue size.
4. `rate(clickhouse_insert_query_total[1m])`: to see the insert rate per minute (how many inserts are sent to clickhouse).
5. `rate(clickhouse_inserted_rows_total[1m])`: to see the rate of rows inserted in clickhouse.
6. `prometheus_remote_storage_dropped_samples_total`: to monitor if any samples are dropped.

## Database Schema

```sql
CREATE TABLE time_series (
    date Date CODEC(Delta),
    fingerprint UInt64,
    labels String
)
ENGINE = ReplacingMergeTree
    PARTITION BY date
    ORDER BY fingerprint;

CREATE TABLE samples (
    fingerprint UInt64,
    timestamp_ms Int64 CODEC(Delta),
    value Float64 CODEC(Delta)
)
ENGINE = MergeTree
    PARTITION BY toDate(timestamp_ms / 1000)
    ORDER BY (fingerprint, timestamp_ms);
```

```sql
SELECT * FROM time_series WHERE fingerprint = 7975981685167825999;
```

```
┌───────date─┬─────────fingerprint─┬─labels─────────────────────────────────────────────────────────────────────────────────┐
│ 2017-12-31 │ 7975981685167825999 │ {"__name__":"up","instance":"promhouse_clickhouse_exporter_1:9116","job":"clickhouse"} │
└────────────┴─────────────────────┴────────────────────────────────────────────────────────────────────────────────────────┘
```

```sql
SELECT * FROM samples WHERE fingerprint = 7975981685167825999 LIMIT 3;
```

```
┌─────────fingerprint─┬──timestamp_ms─┬─value─┐
│ 7975981685167825999 │ 1514730532900 │     0 │
│ 7975981685167825999 │ 1514730533901 │     1 │
│ 7975981685167825999 │ 1514730534901 │     1 │
└─────────────────────┴───────────────┴───────┘
```

Time series in Prometheus are identified by label name/value pairs, including `__name__` label, which stores metric
name. Hash of those pairs is called a fingerprint. PromHouse uses the same hash algorithm as Prometheus to simplify data
migration. During the operation, all fingerprints and label name/value pairs a kept in memory for fast access. The new
time series are written to ClickHouse for persistence. They are also periodically read from it for discovering new time
series written by other ClickHouse instances. `ReplacingMergeTree` ensures there are no duplicates if several ClickHouses
wrote the same time series at the same time.

PromHouse currently stores 24 bytes per sample: 8 bytes for UInt64 fingerprint, 8 bytes for Int64 timestamp, and 8 bytes
for Float64 value. The actual compression rate is about 4.5:1, that's about 24/4.5 = 5.3 bytes per sample. Prometheus
local storage compresses 16 bytes (timestamp and value) per sample to [1.37](https://coreos.com/blog/prometheus-2.0-storage-layer-optimization), that's 12:1.

Since ClickHouse v19.3.3 it is possible to use delta and double delta for compression, which should make storage almost as efficient as TSDB's one.

## Outstanding features in the roadmap

- Downsampling (become possible since ClickHouse v18.12.14)
- Query Hints (become possible since [prometheus PR 4122](https://github.com/prometheus/prometheus/pull/4122), help wanted [issue #24](https://github.com/hmrc/Promhouse/issues/24))

## SQL queries

The largest jobs and instances by time series count:

```sql
SELECT
    job,
    instance,
    COUNT(*) AS value
FROM time_series
GROUP BY
    visitParamExtractString(labels, 'job') AS job,
    visitParamExtractString(labels, 'instance') AS instance
ORDER BY value DESC LIMIT 10
```

The largest metrics by time series count (cardinality):

```sql
SELECT
    name,
    COUNT(*) AS value
FROM time_series
GROUP BY
    visitParamExtractString(labels, '__name__') AS name
ORDER BY value DESC LIMIT 10
```

The largest time series by samples count:

```sql
SELECT
    labels,
    value
FROM time_series
ANY INNER JOIN
(
    SELECT
        fingerprint,
        COUNT(*) AS value
    FROM samples
    GROUP BY fingerprint
    ORDER BY value DESC
    LIMIT 10
) USING (fingerprint)
```
