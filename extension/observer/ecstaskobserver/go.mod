module github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecstaskobserver

go 1.20

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer v0.86.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil v0.86.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.86.0
	github.com/stretchr/testify v1.8.4
	go.opentelemetry.io/collector/component v0.88.0
	go.opentelemetry.io/collector/config/confighttp v0.86.0
	go.opentelemetry.io/collector/confmap v0.88.0
	go.opentelemetry.io/collector/extension v0.86.0
	go.uber.org/zap v1.26.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/klauspost/compress v1.17.0 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/knadh/koanf/v2 v2.0.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20220423185008-bf980b35cac4 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/cors v1.10.0 // indirect
	go.opentelemetry.io/collector v0.86.0 // indirect
	go.opentelemetry.io/collector/config/configauth v0.86.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v0.86.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v0.86.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.88.0 // indirect
	go.opentelemetry.io/collector/config/configtls v0.86.0 // indirect
	go.opentelemetry.io/collector/config/internal v0.86.0 // indirect
	go.opentelemetry.io/collector/extension/auth v0.86.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0017 // indirect
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0017 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.44.0 // indirect
	go.opentelemetry.io/otel v1.19.0 // indirect
	go.opentelemetry.io/otel/metric v1.19.0 // indirect
	go.opentelemetry.io/otel/trace v1.19.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/grpc v1.59.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ../

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil => ../../../internal/aws/ecsutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../../internal/common

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
