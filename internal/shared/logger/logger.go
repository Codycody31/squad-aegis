// Copyright 2023 Woodpecker Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logger

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/6543/logfile-open"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func SetupGlobalLogger(ctx context.Context, logLevel string, pretty bool, noColor bool, logFile string, outputLvl bool) error {
	var file io.ReadWriteCloser
	switch logFile {
	case "", "stderr": // default case
		file = os.Stderr
	case "stdout":
		file = os.Stdout
	default: // a file was set
		openFile, err := logfile.OpenFileWithContext(ctx, logFile, 0o660)
		if err != nil {
			return fmt.Errorf("could not open log file '%s': %w", logFile, err)
		}
		file = openFile
		noColor = true
	}

	log.Logger = zerolog.New(file).With().Timestamp().Logger()

	if pretty {
		log.Logger = log.Output(
			zerolog.ConsoleWriter{
				Out:     file,
				NoColor: noColor,
			},
		)
	}

	lvl, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		return fmt.Errorf("unknown logging level: %s", logLevel)
	}
	zerolog.SetGlobalLevel(lvl)

	// if debug or trace also log the caller
	if zerolog.GlobalLevel() <= zerolog.DebugLevel {
		log.Logger = log.With().Caller().Logger()
	}

	if outputLvl {
		log.Info().Msgf("log level: %s", zerolog.GlobalLevel().String())
	}

	return nil
}
