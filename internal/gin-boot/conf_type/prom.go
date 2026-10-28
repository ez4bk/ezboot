package conf_type

// Monitor Monitor
type Monitor struct {
	StartUp                  bool   `yaml:"_startup"`
	ListenAddr               string `yaml:"listen"`
	MetricPath               string `yaml:"metric"`
	PrometheusServerEndpoint string `yaml:"prometheus_server_endpoint"`
}
