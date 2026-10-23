package conf_type

// RabbitMQ config type
type RabbitMQ struct {
	Startup    bool        `yaml:"_startup"`
	Url        string      `yaml:"url"`
	Publishers []Publisher `yaml:"publishers"`
	Consumers  []Consumer  `yaml:"consumers"`
}

type Publisher struct {
	Name     string    `yaml:"name"`
	Exchange *Exchange `yaml:"exchange"`
}

type Consumer struct {
	Name     string    `yaml:"name"`
	Exchange *Exchange `yaml:"exchange"`
	Queue    *Queue    `yaml:"queue"`
}

type Exchange struct {
	Name       string `yaml:"name"`
	Type       string `yaml:"type"`
	Durable    bool   `yaml:"durable"`
	AutoDelete bool   `yaml:"auto_delete"`
	Internal   bool   `yaml:"internal"`
	NoWait     bool   `yaml:"no_wait"`
}

type Queue struct {
	Name       string `yaml:"name"`
	Durable    bool   `yaml:"durable"`
	AutoDelete bool   `yaml:"auto_delete"`
	Exclusive  bool   `yaml:"exclusive"`
	NoWait     bool   `yaml:"no_wait"`
	Bind       struct {
		BindingKey string `yaml:"binding_key"`
		Exchange   string `yaml:"exchange"`
		NoWait     bool   `yaml:"no_wait"`
	} `yaml:"bind"`
}
