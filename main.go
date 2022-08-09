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
	a, err := app.Create()
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create app")
	}

	var configFile string
	flag.StringVar(&configFile, "config", "config.yaml", "configuration file name")
	flag.Parse()

	c, err := config.ParseConfig(configFile)
	if err != nil {
		a.Logger.Fatal().Err(err).Msgf("cannot parse config file %s", configFile)
	}
	if err := c.Validate(); err != nil {
		a.Logger.Fatal().Err(err).Msg("configuration is not valid")
	}

	a.Logger.Info().Msgf("%s version %s (built: %s)", os.Args[0], Version, BuildDate)
	a.Logger.Info().Msgf("config creation successful, found %d domains", len(c.Checks.Domains))
	a.Logger.Debug().Msgf("%s", c.Checks.Domains)
	a.Logger.Info().Msgf("checks frequency is set to '%s'", c.Checks.Frequency)

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
		// ? is it a good idea? maybe a more granular protection could be better
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

				subl.Info().Msg("getting new TXT records on Cloudflare API")
				vals, err := cloudflare.GetTXTValues(id, ccf)
				if err != nil {
					subl.Error().Err(err).Msg("cannot get new TXT records")
					continue
				}
				subl.Debug().Msgf("got TXT records from Cloudflare: %s", vals)

				// Cloudflare does not return TXT records if a zone does not need
				// a new certificate, so we can continue the loop from here if
				// the condition is matched
				if len(vals) == 0 {
					subl.Info().Msg("Cloudflare did not return any TXT record to use, so this zone probably do not need a certificate renewal. Skipping the next steps...")
					continue
				}

				var txtvalues []string
				for _, v := range vals {
					txtvalues = append(txtvalues, v.TxtValue)
				}
				if !true { // ! debug
					if err := pr.UpdateTXTRecords(subl, d, txtvalues...); err != nil {
						a.Logger.Error().Err(err).Msg("failed to update TXT records")
					}
				}

				if c.Metrics.Enabled {
					subl.Debug().Msg("updating timestamp in last updated metric")
					s.SetDomainLastUpdatedMetric(d)
				}
				subl.Info().Msg("domain records update completed")
			}
		}
	}
}
