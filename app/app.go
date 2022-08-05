package app

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type App struct {
	Logger zerolog.Logger
}

func Create() (*App, error) {
	var a App
	a.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	return &a, nil
}
