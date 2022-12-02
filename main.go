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
	"github.com/rs/zerolog/log"
)

var (
	Version   string
	BuildDate string
)

func main() {
	var runOnce bool
	var dryRun bool
	flag.BoolVar(&runOnce, "run-once", false, "Only one loop over the domains list will be performed.")
	flag.BoolVar(&dryRun, "dry-run", false, "Run in dry mode: no writing action will be performed, only reading.")
	flag.Parse()

	a, err := app.Create()
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create app")
	}
	a.Logger.Info().Msgf("%s version %s (built: %s)", os.Args[0], Version, BuildDate)

	// parse, read and validate the various configuration files
	var configDir string
	flag.StringVar(&configDir, "config-dir", "conf.d", "Path to configuration directory")
	flag.Parse()
	c, err := config.GetConfigFiles(a.Logger, configDir)
	if err != nil {
		a.Logger.Fatal().Err(err).Msgf("cannot parse config file %s", configDir)
	}
	if err := c.Validate(); err != nil {
		a.Logger.Fatal().Err(err).Msg("configuration is not valid")
	}
	a.Logger.Info().Msgf("config creation successful, found %d domains", len(c.Checks.Domains))

	a.Logger.Debug().Msgf("%s", c.Checks.Domains)
	if runOnce {
		a.Logger.Info().Msgf("%s will run once", os.Args[0])
	} else {
		a.Logger.Info().Msgf("checks frequency is set to '%s'", c.Checks.Frequency)
	}

	// start metrics server if asked
	var s *metrics.Server
	if c.Metrics.Enabled {
		s = metrics.Init(c.Metrics.Server.Address, c.Metrics.Server.Port)
		a.Logger.Info().Msgf("starting metrics server on address '%s'", s.Addr)
		go s.Start()
		s.SetNumOfDomainsMetric(len(c.Checks.Domains))
	}

	var ticker time.Ticker
	switch c.Checks.Frequency {
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
		err := fmt.Sprintf("frequency %s is not supported", c.Checks.Frequency)
		a.Logger.Fatal().Err(errors.New(err)).Msg("cannot create ticker")
	}

	// if we only want to run the program once, let's not wait and set a short
	// ticket in order for the loop to be executed almost instantly
	if runOnce {
		ticker = *time.NewTicker(1 * time.Second)
	}

	// detect the correct provider based on the configuration
	var pr providers.Provider
	switch providers.ProviderToUse(*c) {
	case providers.List[providers.OVH]:
		a.Logger.Info().Msg("the detected provider is OVH")
		subdomain := "_acme-challenge"
		covh := ovh.Credentials{
			ApplicationKey:    c.Auth.OVH.AppKey,
			ApplicationSecret: c.Auth.OVH.AppSecret,
			ConsumerKey:       c.Auth.OVH.ConsumerKey,
		}
		pr = ovh.OVHProvider{
			Credentials: covh,
			BaseDomain:  c.Checks.BaseDomain,
			Subdomain:   subdomain,
		}
	case providers.List[providers.NONE]:
		a.Logger.Fatal().Err(errors.New("no provider detected")).
			Msg("no provider detected based on the configuration. Are you sure you completed all the required fields?")
	}

	ccf := cloudflare.Credentials{
		AuthEmail: c.Auth.Cloudflare.Email,
		AuthKey:   c.Auth.Cloudflare.Key,
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
			a.Logger.Debug().Msgf("received ticker signal at %s", t)
			a.Logger.Info().Msg("starting looping around listed domains")

			for _, d := range c.Checks.Domains {
				subl := a.Logger.With().Str("domain", d).Logger()

				subl.Info().Msg("getting zone ID on Cloudflare API")
				id, err := cloudflare.GetZoneID(d, ccf)
				if err != nil {
					subl.Error().Err(err).Msg("cannot get zone ID")
					continue
				}
				subl.Debug().Msgf("got zone ID from Cloudflare: %s", id)

				subl.Info().Msg("checking current certificate packs status")
				status, err := cloudflare.GetCertificatePacksStatus(id, ccf)
				if err != nil {
					subl.Error().Err(err).Msg("cannot check current certificate packs status")
					continue
				}

				if status == cloudflare.ActiveCertificate {
					subl.Info().Msg("certificate packs are active for this domain, trying to cleanup provider's TXT records")
					if dryRun {
						a.Logger.Info().Msg("running in dry-mode, stopping actions now")
						continue
					}

					if err := pr.CleanTXTRecords(subl, d); err != nil {
						subl.Error().Err(err).Msgf("cannot clean certificates for %s", d)
					}
					continue
				}
				subl.Info().Msg("certificate packs are pending for this domain")

				subl.Info().Msg("getting new TXT records on Cloudflare API")
				vals, err := cloudflare.GetTXTValues(id, ccf)
				if err != nil {
					subl.Error().Err(err).Msg("cannot get new TXT records")
					continue
				}
				subl.Debug().Msgf("got TXT records from Cloudflare: %s", vals)

				var txtvalues []string
				for _, v := range vals {
					txtvalues = append(txtvalues, v.TxtValue)
				}

				if dryRun {
					a.Logger.Info().Msg("running in dry-mode, stopping actions now")
					continue
				}

				// before creating the TXT records, we need to ensure that they do not
				// already exist
				ok, err := pr.CheckIfRecordsAlreadyExist(subl, d)
				if err != nil {
					subl.Error().Err(err).Msg("cannot check if TXT records already exist")
					continue
				}

				if ok {
					subl.Info().Msg("TXT records are already set but the certificate packs is still not renewed, so no need to pursue")
					continue
				}

				if err := pr.CreateTXTRecords(subl, d, txtvalues...); err != nil {
					a.Logger.Error().Err(err).Msg("failed to create TXT records")
				}

				if c.Metrics.Enabled {
					subl.Debug().Msg("updating timestamp in last updated metric")
					s.SetDomainLastUpdatedMetric(d)
				}
				subl.Info().Msg("domain records update completed")

			}

			if runOnce {
				a.Logger.Info().Msg("single loop executed, ending the program")
				return
			}
		}
	}
}
