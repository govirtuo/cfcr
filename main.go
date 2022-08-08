package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/govirtuo/cfcr/app"
	"github.com/govirtuo/cfcr/cloudflare"
	"github.com/govirtuo/cfcr/config"
	"github.com/govirtuo/cfcr/metrics"
	"github.com/govirtuo/cfcr/ovh"
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

	// wait and loop
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		// having this pattern allows us to not interrupt the routine execution
		case sig := <-sigs:
			a.Logger.Warn().Msgf("received signal %s, exiting", sig.String())
			os.Exit(1)
		case t := <-ticker.C:
			a.Logger.Debug().Msgf("received ticker signal at %s", t)
			ccf := cloudflare.Credentials{
				AuthEmail: c.Auth.Cloudflare.Email,
				AuthKey:   c.Auth.Cloudflare.Key,
			}

			covh := ovh.Credentials{
				ApplicationKey:    c.Auth.OVH.AppKey,
				ApplicationSecret: c.Auth.OVH.AppSecret,
				ConsumerKey:       c.Auth.OVH.ConsumerKey,
			}

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

				subdomain := "_acme-challenge"
				if d != c.Checks.BaseDomain {
					subdomain = strings.TrimSuffix("_acme-challenge."+d, "."+c.Checks.BaseDomain)
				}

				subl.Info().Msgf("getting IDs for %s records on OVH API", subdomain)
				ids, err := ovh.GetDomainIDs(subdomain, covh)
				if err != nil {
					subl.Error().Err(err).Msg("cannot get IDs")
					continue
				}
				subl.Debug().Msgf("got domain IDs from OVH: %s", ids)

				// TODO: do not update directly! we should compare the TXT records grabbed on Cloudflare
				// and the one that are present on OVH
				for i, v := range vals {
					subl.Info().Msgf("updating %s (ID: %s) with value %s", subdomain, ids[i], v.TxtValue)
					// if err := ovh.UpdateTXTRecord(ids[i], v.TxtValue, subdomain, covh); err != nil {
					// 	subl.Error().Err(err).Msg("cannot update TXT record")
					// }
				}
				if c.Metrics.Enabled {
					s.SetDomainLastUpdatedMetric(d)
				}
				subl.Info().Msg("update completed")
			}
		}
	}
}
