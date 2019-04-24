// +build !windows

package config

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
)

func init() {
	defaultConfFile := filepath.Dir(os.Args[0]) + "/sower.toml"
	if _, err := os.Stat(defaultConfFile); err != nil && os.IsNotExist(err) {
		defaultConfFile = ""
	}

	flag.StringVar(&conf.ConfigFile, "f", defaultConfFile, "config file location")
	printVersion := flag.Bool("v", false, "print sower version")

	if !flag.Parsed() {
		flag.Set("logtostderr", "true")
		flag.Parse()
	}

	if err := conf.Init(); err != nil {
		if *printVersion {
			conf.PrintVersion()
		}
		glog.Exitln(err)
	}

	if *printVersion {
		conf.PrintVersion()
	}
}

// execute run command like console
func execute(command string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "sh", "-c", command).CombinedOutput()
	return errors.Wrapf(err, "cmd: %s, output: %s, error", command, out)
}
