package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/govirtuo/cfcr/app"
	"github.com/govirtuo/cfcr/cloudflare"
	"github.com/govirtuo/cfcr/config"
	"github.com/govirtuo/cfcr/metrics"
	"github.com/govirtuo/cfcr/providers"
	"github.com/govirtuo/cfcr/providers/ovh"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	Version   string
	BuildDate string
)

func main() {
	var runOnce bool
	var dryRun bool
	var configDir string
	flag.BoolVar(&runOnce, "run-once", false, "Only one loop over the domains list will be performed.")
	flag.BoolVar(&dryRun, "dry-run", false, "Run in dry mode: no writing action will be performed, only reading.")
	flag.StringVar(&configDir, "config-dir", "conf.d", "Path to configuration directory.")
	flag.Parse()

	a, err := app.Create()
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create app")
	}
	a.Logger.Info().Msgf("%s version %s (built: %s)", os.Args[0], Version, BuildDate)

	// parse, read and validate the various configuration files
	a.Config, err = config.GetConfigFiles(a.Logger, configDir)
	if err != nil {
		a.Logger.Fatal().Err(err).Msgf("cannot parse config file %s", configDir)
	}
	if err := a.Config.Validate(); err != nil {
		a.Logger.Fatal().Err(err).Msg("configuration is not valid")
	}
	if a.Config.Logging.HumanReadable {
		a.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	a.Logger.Info().Msgf("config creation successful, found %d domains",
		len(a.Config.Checks.Domains))

	a.Logger.Debug().Msgf("%s", a.Config.Checks.Domains)
	if runOnce {
		a.Logger.Info().Msgf("%s will run once", os.Args[0])
	} else {
		a.Logger.Info().Msgf("checks frequency is set to '%s'", a.Config.Checks.Frequency)
	}

	// start metrics server if asked
	if a.Config.Metrics.Enabled {
		a.MetricsServer = metrics.Init(a.Config.Metrics.Server.Address, a.Config.Metrics.Server.Port)
		a.Logger.Info().Msgf("starting metrics server on address '%s'", a.MetricsServer.Addr)
		go func() {
			if err := a.MetricsServer.Start(); err != nil {
				a.Logger.Fatal().Err(err).Msgf("metrics server failed to start")
			}
		}()
		a.MetricsServer.SetNumOfDomainsMetric(len(a.Config.Checks.Domains))
	}

	var ticker time.Ticker
	switch a.Config.Checks.Frequency {
	case "debug":
		ticker = *time.NewTicker(1 * time.Minute)
	case "hourly":
		ticker = *time.NewTicker(time.Hour)
	case "daily":
		ticker = *time.NewTicker(24 * time.Hour)
	case "weekly":
		ticker = *time.NewTicker(24 * 7 * time.Hour)
	case "monthly":
		ticker = *time.NewTicker(24 * 30 * time.Hour) // 30 days a month
	default:
		err := fmt.Sprintf("frequency %s is not supported", a.Config.Checks.Frequency)
		a.Logger.Fatal().Err(errors.New(err)).Msg("cannot create ticker")
	}

	// if we only want to run the program once, let's not wait and set a short
	// ticket in order for the loop to be executed almost instantly
	if runOnce {
		ticker = *time.NewTicker(1 * time.Second)
	}

	// detect the correct provider based on the configuration
	switch providers.ProviderToUse(*a.Config) {
	case providers.List[providers.OVH]:
		a.Logger.Info().Msg("the detected provider is OVH")
		subdomain := "_acme-challenge"
		covh := ovh.Credentials{
			ApplicationKey:    a.Config.Auth.OVH.AppKey,
			ApplicationSecret: a.Config.Auth.OVH.AppSecret,
			ConsumerKey:       a.Config.Auth.OVH.ConsumerKey,
		}
		a.Provider = ovh.OVHProvider{
			Credentials: covh,
			BaseDomain:  a.Config.Checks.BaseDomain,
			Subdomain:   subdomain,
		}
	case providers.List[providers.NONE]:
		a.Logger.Fatal().Err(errors.New("no provider detected")).
			Msg("no provider detected based on the configuration. Are you sure you completed all the required fields?")
	}

	a.CloudflareCredz = cloudflare.Credentials{
		Token: a.Config.Auth.Cloudflare.Token,
	}

	// wait and loop
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		// having this pattern allows us to not interrupt the routine execution
		// with an interrupt signal
		// is it a good idea? maybe a more granular protection could be better
		case sig := <-sigs:
			a.Logger.Warn().Msgf("received signal %s, exiting", sig.String())
			os.Exit(1)
		case t := <-ticker.C:
			if err := a.Run(t, dryRun); err != nil {
				a.Logger.Fatal().Msgf("error running app: %s", err)
			}
			if runOnce {
				return
			}
		}
	}
}
