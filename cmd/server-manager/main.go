package main

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/lorenzosaino/go-sysctl"
	_ "github.com/mjibson/esc/embed"
	"github.com/pkg/browser"
	"github.com/sirupsen/logrus"

	"justapengu.in/acsm"
	"justapengu.in/acsm/cmd/server-manager/static"
	"justapengu.in/acsm/cmd/server-manager/views"
	"justapengu.in/acsm/internal/changelog"
	"justapengu.in/acsm/pkg/license"
)

var defaultAddress = "0.0.0.0:8772"

const (
	udpRealtimePosRefreshIntervalMin = 50
)

func init() {
	runtime.LockOSThread()
	acsm.InitLogging()
}

func main() {
	if err := license.LoadAndValidateLicense(license.Filename); err != nil {
		logrus.WithError(err).Fatal("Failed to validate license")
		return
	}

	l := license.GetLicense()

	if !l.Expires.IsZero() {
		go func() {
			tick := time.Tick(time.Minute * 10)

			for range tick {
				if l.Expires.Before(time.Now()) {
					logrus.Fatalf("License has expired")
					return
				}
			}
		}()
	}

	config, err := acsm.ReadConfig("config.yml")

	if err != nil {
		ServeHTTPWithError(defaultAddress, "Read configuration file (config.yml)", err)
		return
	}

	if config.Monitoring.Enabled {
		acsm.InitMonitoring()
	}

	store, err := config.Store.BuildStore()

	if err != nil {
		ServeHTTPWithError(config.HTTP.Hostname, "Open server manager storage (bolt or json)", err)
		return
	}

	changes, err := changelog.LoadChangelog()

	if err != nil {
		ServeHTTPWithError(config.HTTP.Hostname, "Load changelog (internal error)", err)
		return
	}

	acsm.Changelog = changes

	var templateLoader acsm.TemplateLoader
	var filesystem http.FileSystem

	if os.Getenv("FILESYSTEM_HTML") == "true" {
		templateLoader = acsm.NewFilesystemTemplateLoader("views")
		filesystem = http.Dir("static")
	} else {
		templateLoader = &views.TemplateLoader{}
		filesystem = static.FS(false)
	}

	resolver, err := acsm.NewResolver(templateLoader, os.Getenv("FILESYSTEM_HTML") == "true", store)

	if err != nil {
		ServeHTTPWithError(config.HTTP.Hostname, "Initialise resolver (internal error)", err)
		return
	}

	acsm.SetAssettoInstallPath(config.Steam.InstallPath)

	err = acsm.InstallAssettoCorsaServerWithSteamCMD(config.Steam.Username, config.Steam.Password, config.Steam.ForceUpdate)

	if err != nil {
		logrus.Warnf("Could not find or install Assetto Corsa Server using SteamCMD. Creating barebones install.")

		if err := acsm.InstallBareBonesAssettoCorsaServer(); err != nil {
			ServeHTTPWithError(defaultAddress, "Install assetto corsa server with steamcmd or using barebones install.", err)
			return
		}
	}

	if config.LiveMap.IsEnabled() {
		if config.LiveMap.IntervalMs < udpRealtimePosRefreshIntervalMin {
			acsm.RealtimePosInterval = time.Duration(udpRealtimePosRefreshIntervalMin) * time.Millisecond
		} else {
			acsm.RealtimePosInterval = time.Duration(config.LiveMap.IntervalMs) * time.Millisecond
		}

		if runtime.GOOS == "linux" {
			// check known kernel net memory restrictions. if they're lower than the recommended
			// values, then print out explaining how to increase them
			memValues := []string{"net.core.rmem_max", "net.core.rmem_default", "net.core.wmem_max", "net.core.wmem_default"}

			for _, val := range memValues {
				checkMemValue(val)
			}
		}
	}

	if config.Lua.Enabled {
		luaPath := os.Getenv("LUA_PATH")

		newPath, err := filepath.Abs("./plugins/?.lua")

		if err != nil {
			logrus.WithError(err).Error("Couldn't get absolute path for /plugins folder")
		} else {
			if luaPath != "" {
				luaPath = luaPath + ";" + newPath
			} else {
				luaPath = newPath
			}

			err = os.Setenv("LUA_PATH", luaPath)

			if err != nil {
				logrus.WithError(err).Error("Couldn't automatically set Lua path, lua will not run! Try setting the environment variable LUA_PATH manually.")
			}
		}

		acsm.InitLua(resolver.ResolveRaceControl())
	}

	err = acsm.InitWithResolver(resolver)

	if err != nil {
		ServeHTTPWithError(config.HTTP.Hostname, "Initialise server manager (internal error)", err)
		return
	}

	listener, err := net.Listen("tcp", config.HTTP.Hostname)

	if err != nil {
		ServeHTTPWithError(defaultAddress, "Listen on hostname "+config.HTTP.Hostname+". Likely the port has already been taken by another application", err)
		return
	}

	logrus.Infof("starting assetto server manager on: %s", config.HTTP.Hostname)

	if !config.Server.DisableWindowsBrowserOpen && runtime.GOOS == "windows" {
		_ = browser.OpenURL("http://" + strings.Replace(config.HTTP.Hostname, "0.0.0.0", "127.0.0.1", 1))
	}

	router := resolver.ResolveRouter(filesystem)

	srv := &http.Server{
		Handler: router,
	}

	if config.HTTP.TLS.Enabled {
		srv.TLSConfig = &tls.Config{
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
		}

		srv.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler))

		if err := srv.ServeTLS(listener, config.HTTP.TLS.CertPath, config.HTTP.TLS.KeyPath); err != nil {
			logrus.WithError(err).Fatal("Could not start TLS server")
		}
	} else {
		if err := srv.Serve(listener); err != nil {
			logrus.WithError(err).Fatal("Could not start server")
		}
	}
}

const udpBufferRecommendedSize = uint64(2e6) // 2MB

func checkMemValue(key string) {
	val, err := sysctlAsUint64(key)

	if err != nil {
		logrus.WithError(err).Errorf("Could not check sysctl val: %s", key)
		return
	}

	if val < udpBufferRecommendedSize {
		d := color.New(color.FgRed)
		red := d.PrintfFunc()
		redln := d.PrintlnFunc()

		redln()
		redln("-------------------------------------------------------------------")
		redln("                          W A R N I N G")
		redln("-------------------------------------------------------------------")

		red("System %s value is too small! UDP messages are \n", key)
		redln("more likely to be lost and the stability of various Server Manager")
		redln("systems will be greatly affected.")
		redln()

		red("Your current value is %s. We recommend a value of %s for a \n", humanize.Bytes(val), humanize.Bytes(udpBufferRecommendedSize))
		redln("more consistent operation.")
		redln()

		red("You can do this with the command:\n\t sysctl -w %s=%d\n", key, udpBufferRecommendedSize)
		redln()

		redln("More information can be found on sysctl variables here:\n\t https://www.cyberciti.biz/faq/howto-set-sysctl-variables/")
	}
}

func sysctlAsUint64(val string) (uint64, error) {
	val, err := sysctl.Get(val)

	if err != nil {
		return 0, err
	}

	return strconv.ParseUint(val, 10, 0)
}
