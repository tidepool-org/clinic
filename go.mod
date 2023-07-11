module github.com/tidepool-org/clinic

go 1.19

require (
	github.com/deepmap/oapi-codegen v1.12.4
	github.com/fatih/structs v1.1.0
	github.com/getkin/kin-openapi v0.117.0
	github.com/golang/mock v1.5.0
	github.com/hashicorp/golang-lru v0.5.3
	github.com/jaswdr/faker v1.4.2
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/labstack/echo/v4 v4.9.1
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/open-policy-agent/opa v0.27.1
	github.com/tidepool-org/clinic/client v0.0.0-00010101000000-000000000000
	github.com/tidepool-org/clinic/redox/models v0.0.0-00010101000000-000000000000
	github.com/tidepool-org/go-common v0.8.3-0.20210528114116-26ab9a2d32b5
	go.mongodb.org/mongo-driver v1.12.0
	go.uber.org/fx v1.13.1
	go.uber.org/zap v1.13.0
)

require (
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/OneOfOne/xxhash v1.2.8 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/bytecodealliance/wasmtime-go v0.24.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/swag v0.21.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/golang/snappy v0.0.1 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/hpcloud/tail v1.0.0 // indirect
	github.com/invopop/yaml v0.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/labstack/gommon v0.4.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.16 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/montanaflynn/stats v0.0.0-20171201202039-1bf9dbcd8cbe // indirect
	github.com/perimeterx/marshmallow v1.1.4 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20200313005456-10cdbea86bc0 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/yashtewari/glob-intersection v0.0.0-20180916065949-5c77d914dd0b // indirect
	github.com/youmark/pkcs8 v0.0.0-20181117223130-1be2e3e5546d // indirect
	go.uber.org/atomic v1.5.0 // indirect
	go.uber.org/dig v1.10.0 // indirect
	go.uber.org/multierr v1.4.0 // indirect
	go.uber.org/tools v0.0.0-20190618225709-2cfd321de3ee // indirect
	golang.org/x/crypto v0.1.0 // indirect
	golang.org/x/lint v0.0.0-20201208152925-83fdc39ff7b5 // indirect
	golang.org/x/mod v0.7.0 // indirect
	golang.org/x/net v0.2.0 // indirect
	golang.org/x/sync v0.1.0 // indirect
	golang.org/x/sys v0.2.0 // indirect
	golang.org/x/text v0.7.0 // indirect
	golang.org/x/time v0.0.0-20220411224347-583f2d630306 // indirect
	golang.org/x/tools v0.3.0 // indirect
	golang.org/x/xerrors v0.0.0-20220411194840-2f41105eb62f // indirect
	gopkg.in/fsnotify.v1 v1.4.7 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	honnef.co/go/tools v0.0.1-2019.2.3 // indirect
)

replace github.com/tidepool-org/clinic/client => ./client

replace github.com/tidepool-org/clinic/redox/models => ./redox/models
