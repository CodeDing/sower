package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/golang/glog"
	toml "github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"github.com/wweir/sower/util"
)

var (
	Version, Date string
	conf          = &Conf{}
	// OnRefresh execute hooks while refesh config
	OnRefresh = []func(*Conf) (string, error){
		func(c *Conf) (action string, err error) {
			action = "clear dns cache"

			if c.DNS.Suggest.OnSuggest != "" {
				err = execute(c.DNS.Suggest.OnSuggest)
			}
			return
		},
	}
)

func GetConf() *Conf {
	return conf
}

// config definition
type Conf struct {
	ConfigFile string `toml:"-"`
	Verbose    int    `toml:"verbose"`

	DNS struct {
		UpstreamDNS string `toml:"upstream_dns"`
		RedirectIP  string `toml:"redirect_ip"`

		Rule struct {
			BlockList []string `toml:"blocklist"`
			WhiteList []string `toml:"whitelist"`
		} `toml:"rule"`
		Suggest struct {
			SuggestLevel string   `toml:"suggest_level"`
			OnSuggest    string   `toml:"on_suggest"`
			Suggestions  []string `toml:"suggestions"`
		} `toml:"suggest"`
	} `toml:"dns"`

	P2P struct {
		Peer struct {
			ID        string `toml:"id"`
			Transport string `toml:"transport"`
			Cipher    string `toml:"cipher"`
			Password  string `toml:"password"`
			PunchAddr string `toml:"punch_addr"`
		} `toml:"peer"`

		Target struct {
			ID        string `toml:"id"`
			Transport string `toml:"transport"`
			Cipher    string `toml:"cipher"`
			Password  string `toml:"password"`
		} `toml:"target"`

		PunchServer struct {
			Addr     string `toml:"addr"`
			Cipher   string `toml:"cipher"`
			Password string `toml:"password"`
		} `toml:"punch_server"`
	} `toml:"p2p"`

	Proxy struct {
		HTTPProxy  string   `toml:"http_proxy"`
		RelayPorts []string `toml:"relay_ports"`
	} `toml:"proxy"`
}

func (c *Conf) Init() error {
	f, err := os.OpenFile(c.ConfigFile, os.O_RDONLY, 0644)
	if err != nil {
		return errors.Wrapf(err, "load config (%s)", c.ConfigFile)
	}
	defer f.Close()

	file := c.ConfigFile // keep config file path
	if err = toml.NewDecoder(f).Decode(c); err != nil {
		return errors.Wrapf(err, "decode config (%s) fail: %s", c.ConfigFile)
	}
	c.ConfigFile = file

	for i := range OnRefresh {
		if action, err := OnRefresh[i](c); err != nil {
			return errors.Wrap(err, action)
		}
	}

	return nil
}

// AddSuggestion add new domain into suggest rules
func (c *Conf) AddSuggestion(domain string) {
	// concurrency unsafe, acceptable
	c.DNS.Suggest.Suggestions = append(c.DNS.Suggest.Suggestions, domain)
	c.DNS.Suggest.Suggestions = util.NewReverseSecSlice(c.DNS.Suggest.Suggestions).Sort().Uniq()

	if err := c.Store(); err != nil {
		glog.Errorln(err)
	}

	// reload config
	for i := range OnRefresh {
		if action, err := OnRefresh[i](c); err != nil {
			glog.Errorln(action+":", err)
		}
	}
}

func (c *Conf) Store() error {
	f, err := os.OpenFile(c.ConfigFile+"~", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return errors.Wrapf(err, "open %s~", c.ConfigFile)
	}
	defer func() {
		f.Close()
		os.Remove(c.ConfigFile + "~")
	}()

	if err := toml.NewEncoder(f).ArraysWithOneElementPerLine(true).Encode(c); err != nil {
		return errors.Wrap(err, "encode config")
	}

	err = os.Rename(c.ConfigFile+"~", c.ConfigFile)
	return errors.Wrapf(err, "move config %s", c.ConfigFile)
}

func (c *Conf) PrintVersion() {
	config, _ := json.MarshalIndent(conf, "", "\t")
	fmt.Printf("Version:\n\t%s %s\nConfig:\n%s", Version, Date, config)
	os.Exit(0)
}
