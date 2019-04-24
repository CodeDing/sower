// +build windows

package config

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

func init() {
	defaultConfFile := filepath.Dir(os.Args[0]) + "\\sower.toml"
	if _, err := os.Stat(defaultConfFile); err != nil && os.IsNotExist(err) {
		defaultConfFile = ""
	}

	flag.StringVar(&conf.ConfigFile, "f", defaultConfFile, "config file location")
	printVersion := flag.Bool("v", false, "print sower version")
	install := flag.Bool("install", false, "install sower as a service")
	uninstall := flag.Bool("uninstall", false, "uninstall sower from service list")

	if !flag.Parsed() {
		os.Mkdir("log", 0755)
		flag.Set("log_dir", filepath.Dir(os.Args[0])+"/log")
		flag.Parse()
	}

	if err := conf.Init(); err != nil {
		if *printVersion {
			conf.PrintVersion()
		}
		glog.Exitln(err)
	}

	switch {
	case *printVersion:
		conf.PrintVersion()

	case *install:
		installService()
		os.Exit(0)

	case *uninstall:
		uninstallService()
		os.Exit(0)

	default:
		runWithService()
	}
}

// Windows service implement
const name = "sower"
const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue

func runWithService() {
	os.Chdir(filepath.Dir(os.Args[0]))
	if active, err := svc.IsAnInteractiveSession(); err != nil {
		glog.Exitf("failed to determine if we are running in an interactive session: %v", err)
	} else if !active {
		go func() {
			elog, err := eventlog.Open(name)
			if err != nil {
				glog.Exitln(err)
			}
			defer elog.Close()

			if err := svc.Run(name, &myservice{}); err != nil {
				elog.Error(1, fmt.Sprintf("%s service failed: %v", name, err))
				glog.Exitln(err)
			}
			elog.Info(1, fmt.Sprintf("winsvc.RunAsService: %s service stopped", name))
			os.Exit(0)
		}()
	}
}
func installService() {
	mgrDo(func(m *mgr.Mgr) error {
		s, err := m.OpenService(name)
		if err == nil {
			s.Close()
			return fmt.Errorf("service %s already exists", name)
		}
		s, err = m.CreateService(name, os.Args[0], mgr.Config{
			DisplayName: "Sower Proxy",
			StartType:   windows.SERVICE_AUTO_START,
		})
		if err != nil {
			return err
		}
		defer s.Close()
		err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
		if err != nil {
			s.Delete()
			return fmt.Errorf("SetupEventLogSource() failed: %s", err)
		}

		return s.Start()
	})
}

func uninstallService() {
	serviceDo(func(s *mgr.Service) error {
		err := s.Delete()
		if err != nil {
			return err
		}
		return eventlog.Remove(name)
	})
}

func serviceDo(fn func(*mgr.Service) error) {
	mgrDo(func(m *mgr.Mgr) error {
		s, err := m.OpenService(name)
		if err != nil {
			return fmt.Errorf("could not access service: %v", err)
		}
		defer s.Close()
		return fn(s)
	})
}
func mgrDo(fn func(m *mgr.Mgr) error) {
	m, err := mgr.Connect()
	if err != nil {
		glog.Exitln(err)
	}
	defer m.Disconnect()

	if err := fn(m); err != nil {
		glog.Fatalln(err)
	}
}

type myservice struct{}

func (m *myservice) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	elog, err := eventlog.Open(name)
	if err != nil {
		glog.Errorln(err)
		return
	}
	defer elog.Close()
	elog.Info(1, strings.Join(args, "-"))

	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	for {
		c := <-r
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
			// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
			time.Sleep(100 * time.Millisecond)
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			return
		case svc.Pause:
			changes <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}
		case svc.Continue:
			changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
		default:
			elog.Error(1, fmt.Sprintf("unexpected control request #%d", c))
		}
	}
}

// execute run command like console
func execute(command string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	var cmds []string
	for _, cmd := range strings.Split(command, " ") {
		if cmd == "" {
			continue
		}
		if strings.HasPrefix(cmd, "/") {
			cmd = strings.Replace(cmd, "/", "-", 1)
		}
		cmds = append(cmds, cmd)
	}

	if len(cmds) != 0 {
		return nil
	}

	cmd := exec.CommandContext(ctx, cmds[0], cmds[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	return errors.Wrapf(err, "cmd: %s, output: %s, error", command, out)
}
