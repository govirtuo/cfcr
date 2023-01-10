package app

import (
	"os"
	"time"

	"github.com/govirtuo/cfcr/cloudflare"
	"github.com/govirtuo/cfcr/config"
	"github.com/govirtuo/cfcr/metrics"
	"github.com/govirtuo/cfcr/providers"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// App is a wrap struct around all the main config and and values that need to
// be shared across the program.
type App struct {
	Logger          zerolog.Logger
	Config          *config.Config
	Provider        providers.Provider
	CloudflareCredz cloudflare.Credentials

	MetricsServer *metrics.Server
}

// Create creates a new App with an initialized logger only
func Create() (*App, error) {
	var a App
	a.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	return &a, nil
}

func (a App) Run(t time.Time, dryRun bool) error {
	a.Logger.Debug().Msgf("received ticker signal at %s", t)
	a.Logger.Info().Msg("starting looping around listed domains")

	for _, d := range a.Config.Checks.Domains {
		subl := a.Logger.With().Str("domain", d).Logger()

		subl.Info().Msg("getting zone ID on Cloudflare API")
		id, err := cloudflare.GetZoneID(d, a.CloudflareCredz)
		if err != nil {
			subl.Error().Err(err).Msg("cannot get zone ID")
			continue
		}
		subl.Debug().Msgf("got zone ID from Cloudflare: %s", id)

		subl.Info().Msg("checking current certificate packs status")
		status, err := cloudflare.GetCertificatePacksStatus(id, a.CloudflareCredz)
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

			if err := a.Provider.CleanTXTRecords(subl, d); err != nil {
				subl.Error().Err(err).Msgf("cannot clean certificates for %s", d)
			}
			continue
		}
		subl.Info().Msg("certificate packs are pending for this domain")

		subl.Info().Msg("getting new TXT records on Cloudflare API")
		vals, err := cloudflare.GetTXTValues(id, a.CloudflareCredz)
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
		ok, err := a.Provider.CheckIfRecordsAlreadyExist(subl, d)
		if err != nil {
			subl.Error().Err(err).Msg("cannot check if TXT records already exist")
			continue
		}

		if ok {
			subl.Info().Msg("TXT records are already set but the certificate packs is still not renewed, so no need to pursue")
			continue
		}

		if err := a.Provider.CreateTXTRecords(subl, d, txtvalues...); err != nil {
			a.Logger.Error().Err(err).Msg("failed to create TXT records")
			continue
		}

		if a.Config.Metrics.Enabled {
			subl.Debug().Msg("updating timestamp in last updated metric")
			a.MetricsServer.SetDomainLastUpdatedMetric(d)
		}
		subl.Info().Msg("domain records update completed")

	}

	return nil
}
