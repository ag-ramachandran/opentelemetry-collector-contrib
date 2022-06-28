# Azure Monitor Exporter

This exporter sends metrics , logs and trace data to [Azure Data Explorer](https://azure.microsoft.com/en-us/services/data-explorer/).

## Configuration

The following settings are required:

- `cluster_name` (no default): The cluster name of the provisioned ADX cluster to ingest the data.
- `client_id` (no default): The client id to connect to the cluster and ingest data.
- `client_secret` (no default): The cluster secret corresponding to the client id.
- `tenant_id` (no default): The tenant id where the client_id is referenced from.

The following settings can be optionally configured and have default values:
#### Note that the database , tables are expected to be created upfront before the exporter is in operation , the definition of these are in the section [Database and Table definition scripts](#database-and-table-definition-scripts)

- `db_name` (default = "oteldb"): The ADX database where the tables are present to ingest the data.
- `metrics_table_name` (default = OTELMetrics): The target table in the database `db_name` that stores exported metric data.
- `logs_table_name` (default = OTELLogs): The target table in the database `db_name` that stores exported logs data.
- `traces_table_name` (default = OTELLogs): The target table in the database `db_name` that stores exported traces data.
- `ingestion_type` (default = queued): ADX ingest can happen in managed [streaming](https://docs.microsoft.com/en-us/azure/data-explorer/kusto/management/streamingingestionpolicy) or [queued](https://docs.microsoft.com/en-us/azure/data-explorer/kusto/management/batchingpolicy) modes.

An example configuration is provided as follows:

```yaml
exporters:
  azuredataexplorer:
    # Kusto cluster uri
    cluster_name: "https://CLUSTER.kusto.windows.net"
    # Client Id
    client_id: "f80da32c-108c-415c-a19e-643f461a677a"
    # The client secret for the client
    client_secret: "17cc3f47-e95e-4045-af6c-ec2eea163cc6"
    # The tenant
    tenant_id: "21ff9e36-fbaa-43c8-98ba-00431ea10bc3"
    # database for the logs
    db_name: "oteldb"
    # raw metric table name
    metrics_table_name: "RawMetrics"
    # raw log table name
    logs_table_name: "RawLogs"
     # raw traces table
    traces_table_name: "RawTraces"
    # type of ingestion managed or queued
    ingestion_type : "managed"
```

## Attribute mapping

### Metrics

This exporter maps [OpenTelemetry metric attributes](https://opentelemetry.io/docs/reference/specification/metrics/sdk/) to specific structures in the ADX tables. These can then be extended by usage of [update policies](https://docs.microsoft.com/en-us/azure/data-explorer/kusto/management/updatepolicy) in ADX if needed

| ADX Table column              | Description / OpenTelemetry attribute                               
| ----------------------------- |------------------------------------------------------ 
| Timestamp                     | The timestamp of the datapoint            
| MetricName                    | The name of the datapoint          
| MetricType                    | The type / datapoint type of the metric (e.g. Sum, Histogram, Gauge etc.)
| MetricUnit                    | Unit of measure of the metric
| MetricDescription             | Description about the metric
| MetricValue                   | The metric value measured for the datapoint
| MetricAttributes              | Custom metric [attributes](https://opentelemetry.io/docs/reference/specification/common/#attribute) set from the application. Also contains the [instrumentation scope](https://opentelemetry.io/docs/reference/specification/common/#attribute) name and version  
| Host                          | The host.name extracted from [Host resource semantics](https://opentelemetry.io/docs/reference/specification/resource/semantic_conventions/host/). If empty , the hostname of the exporter is used
| ResourceAttributes            | The resource attributes JSON map as specified in open telemetry [resource semantics](https://opentelemetry.io/docs/reference/specification/resource/semantic_conventions/)


### Database and Table definition scripts

The following tables need to be created in the database specified in the configuration.

```sql
.create-merge table OTELLogs (Timestamp:datetime, ObservedTimestamp:datetime, TraceId:string, SpanId:string, SeverityText:string, SeverityNumber:int, Body:string, ResourceAttributes:dynamic, LogsAttributes:dynamic) 

.create-merge table OTELMetrics (Timestamp:datetime, MetricName:string, MetricType:string, MetricUnit:string, MetricDescription:string, MetricValue:real, Host:string, ResourceAttributes:dynamic,MetricAttributes:dynamic) 

.create-merge table OTELTraces (TraceId:string, SpanId:string, ParentId:string, SpanName:string, SpanStatus:string, SpanKind:string, StartTime:datetime, EndTime:datetime, ResourceAttributes:dynamic, TraceAttributes:dynamic, Events:dynamic, Links:dynamic) 
```