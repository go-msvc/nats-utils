package nats

import (
	"net/url"
	"time"

	"github.com/go-msvc/config"
	"github.com/go-msvc/errors"
	"github.com/go-msvc/logger"
	"github.com/go-msvc/nats-utils/datatype"
	"github.com/nats-io/nats.go"
)

var log = logger.New() //.WithLevel(logger.LevelDebug)

func init() {
	config.RegisterConstructor("nats", ServerConfig{
		Config: Config{
			Domain:         "", //must be configured
			Url:            "nats://localhost:4222",
			ConnectTimeout: datatype.Duration(time.Second * 2),
		},
	})
}

// common config used by both client and server to connect to the nats service
type Config struct {
	Domain             string            `json:"domain" doc:"NATS client name that will be used for subscription on '<domain>.*', e.g. use 'ussd'"`
	Url                string            `json:"url" doc:"NATS connection URL, defaults to 'nats://127.0.0.1:4222'"`
	ConnectTimeout     datatype.Duration `json:"connect_timeout"`
	MaxReconnects      int               `json:"max_reconnects"`
	ReconnectWait      datatype.Duration `json:"reconnect_wait"`
	ReconnectJitter    datatype.Duration `json:"reconnect_jitter"`
	ReconnectJitterTls datatype.Duration `json:"reconnect_jitter_tls"`
	DontRandomize      bool              `json:"dont_randomize"`
	Username           string            `json:"username"`
	Password           datatype.EncStr   `json:"password"`
	Token              string            `json:"token"`
	Secure             bool              `json:"secure"`
	InsecureSkipVerify bool              `json:"insecure_skip_verify"`
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.Errorf("nil.Validate()")
	}
	if len(c.Domain) <= 0 {
		return errors.Errorf("missing domain")
	}
	if len(c.Url) <= 0 {
		c.Url = nats.DefaultURL
	}
	if pu, err := url.ParseRequestURI(c.Url); err != nil {
		return errors.Wrapf(err, "invalid url:\"%s\"")
	} else {
		if pu.Scheme != "nats" {
			return errors.Errorf("url:\"%s\" must have scheme \"nats://...\", not \"%s://...\"", c.Url, pu.Scheme)
		}
	}
	if c.MaxReconnects == 0 {
		c.MaxReconnects = 10
	}
	if c.MaxReconnects < 0 {
		return errors.Errorf("invalid max_reconnects:%d", c.MaxReconnects)
	}
	if c.ReconnectWait == 0 {
		c.ReconnectWait = datatype.Duration(time.Second * 2)
	}
	if c.ReconnectWait < 0 {
		return errors.Errorf("invalid reconnect_wait:\"%s\"", c.ReconnectWait)
	}
	return nil
} //Config.Validate()
