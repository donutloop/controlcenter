// SPDX-FileCopyrightText: 2021 Lightmeter <hello@lightmeter.io>
//
// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"flag"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	uuid "github.com/satori/go.uuid"
	"gitlab.com/lightmeter/controlcenter/lmsqlite3"
	"gitlab.com/lightmeter/controlcenter/logeater/announcer"
	"gitlab.com/lightmeter/controlcenter/logeater/dirlogsource"
	"gitlab.com/lightmeter/controlcenter/logeater/filelogsource"
	"gitlab.com/lightmeter/controlcenter/logeater/logsource"
	"gitlab.com/lightmeter/controlcenter/logeater/socketsource"
	"gitlab.com/lightmeter/controlcenter/logeater/transform"
	"gitlab.com/lightmeter/controlcenter/server"
	"gitlab.com/lightmeter/controlcenter/subcommand"
	"gitlab.com/lightmeter/controlcenter/util/errorutil"
	"gitlab.com/lightmeter/controlcenter/version"
	"gitlab.com/lightmeter/controlcenter/workspace"
	"os"
	"time"
)

func main() {
	var (
		shouldWatchFromStdin      bool
		workspaceDirectory        string
		importOnly                bool
		rsyncedDir                bool
		migrateDownToOnly         bool
		migrateDownToVersion      int
		migrateDownToDatabaseName string
		showVersion               bool
		dirToWatch                string
		address                   string
		verbose                   bool
		emailToPasswdReset        string
		passwordToReset           string
		timezone                  *time.Location = time.UTC
		logYear                   int
		socket                    string
		logFormat                 string
	)

	flag.BoolVar(&shouldWatchFromStdin, "stdin", false, "Read log lines from stdin")
	flag.StringVar(&workspaceDirectory, "workspace", "/var/lib/lightmeter_workspace", "Path to the directory to store all working data")
	flag.BoolVar(&importOnly, "importonly", false,
		"Only import existing logs, exiting immediately, without running the full application.")
	flag.BoolVar(&rsyncedDir, "rsync", false, "Log directory is updated by rsync")
	flag.BoolVar(&migrateDownToOnly, "migrate_down_to_only", false,
		"Only migrates down")
	flag.StringVar(&migrateDownToDatabaseName, "migrate_down_to_database", "", "Database name only for migration")
	flag.IntVar(&migrateDownToVersion, "migrate_down_to_version", -1, "Specify the new migration version")
	flag.IntVar(&logYear, "log_starting_year", 0, "Value to be used as initial year when it cannot be obtained from the Postfix logs. Defaults to the current year. Requires -stdin.")
	flag.BoolVar(&showVersion, "version", false, "Show Version Information")
	flag.StringVar(&dirToWatch, "watch_dir", "", "Path to the directory where postfix stores its log files, to be watched")
	flag.StringVar(&address, "listen", ":8080", "Network address to listen to")
	flag.BoolVar(&verbose, "verbose", false, "Be Verbose")
	flag.StringVar(&emailToPasswdReset, "email_reset", "", "Reset password for user (implies -password and depends on -workspace)")
	flag.StringVar(&passwordToReset, "password", "", "Password to reset (requires -email_reset)")
	flag.StringVar(&socket, "socket", "", "Receive logs via a socket. E.g. unix=/tmp/lightemter.sock or tcp=localhost:9999")
	flag.StringVar(&logFormat, "log_format", "default", "Expected log format from external sources (like logstash, etc.)")

	flag.Usage = func() {
		printVersion()
		fmt.Fprintf(os.Stdout, "\n Example call: \n")
		fmt.Fprintf(os.Stdout, "\n %s -workspace ~/lightmeter_workspace -watch_dir /var/log \n", os.Args[0])
		fmt.Fprintf(os.Stdout, "\n Flag set: \n\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).With().Str("service", "controlcenter").Str("instanceid", uuid.NewV4().String()).Caller().Logger()

	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	if verbose {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	if showVersion {
		printVersion()
		return
	}

	liabilityDisclamer := `This program comes with ABSOLUTELY NO WARRANTY. This is free software, and you are welcome to redistribute it under certain conditions; see here for details: https://lightmeter.io/lmcc-license.`

	log.Info().Msg(liabilityDisclamer)

	lmsqlite3.Initialize(lmsqlite3.Options{})

	if migrateDownToOnly {
		subcommand.PerformMigrateDownTo(verbose, workspaceDirectory, migrateDownToDatabaseName, int64(migrateDownToVersion))
		return
	}

	if len(emailToPasswdReset) > 0 {
		subcommand.PerformPasswordReset(verbose, workspaceDirectory, emailToPasswdReset, passwordToReset)
		return
	}

	ws, err := workspace.NewWorkspace(workspaceDirectory)

	if err != nil {
		errorutil.Dief(verbose, errorutil.Wrap(err), "Error creating / opening workspace directory for storing application files: %s. Try specifying a different directory (using -workspace), or check you have permission to write to the specified location.", workspaceDirectory)
	}

	logSource, err := buildLogSource(ws, dirToWatch, importOnly, rsyncedDir, logYear, logFormat, shouldWatchFromStdin, socket, verbose)

	if err != nil {
		errorutil.Dief(verbose, err, "Error setting up logs reading")
	}

	done, cancel := ws.Run()

	logReader := logsource.NewReader(logSource, ws.NewPublisher())

	// only import logs and exit when they end. Does not start web server.
	// It's useful for benchmarking importing logs.
	if importOnly {
		err := logReader.Run()

		if err != nil {
			errorutil.Dief(verbose, err, "Error reading logs")
		}

		cancel()

		err = done()

		errorutil.MustSucceed(err)

		log.Info().Msg("Importing has finished. Bye!")

		return
	}

	// from here on, workspace is never cancellable!

	go func() {
		err := done()
		errorutil.Dief(verbose, err, "Error: Workspace execution has ended, which should never happen here!")
	}()

	go func() {
		err := logReader.Run()
		if err != nil {
			errorutil.Dief(verbose, err, "Error reading logs")
		}
	}()

	httpServer := server.HttpServer{
		Workspace:          ws,
		WorkspaceDirectory: workspaceDirectory,
		Timezone:           timezone,
		Address:            address,
	}

	errorutil.MustSucceed(httpServer.Start(), "server died")
}

func printVersion() {
	//nolint:forbidigo
	fmt.Printf("Lightmeter ControlCenter %s\n", version.Version)
}

func importAnnouncerOnlyForFirstExecution(initialTime time.Time, a announcer.ImportAnnouncer) announcer.ImportAnnouncer {
	// first execution. Must import historical insights
	if initialTime.IsZero() {
		return a
	}

	// otherwise skip the historical insights import
	return announcer.Skipper(a)
}

func buildLogSource(ws *workspace.Workspace, dirToWatch string, importOnly bool, rsyncedDir bool, logYear int, logFormat string, shouldWatchFromStdin bool, socket string, verbose bool) (logsource.Source, error) {
	mostRecentTime, err := ws.MostRecentLogTime()
	if err != nil {
		return nil, errorutil.Wrap(err)
	}

	announcer := importAnnouncerOnlyForFirstExecution(mostRecentTime, ws.ImportAnnouncer())

	if len(dirToWatch) > 0 {
		s, err := dirlogsource.New(dirToWatch, mostRecentTime, announcer, !importOnly, rsyncedDir)
		if err != nil {
			return nil, errorutil.Wrap(err)
		}

		return s, nil
	}

	builder, err := transform.Get(logFormat, logYear)
	if err != nil {
		return nil, errorutil.Wrap(err)
	}

	if shouldWatchFromStdin {
		s, err := filelogsource.New(os.Stdin, builder, announcer)
		if err != nil {
			return nil, errorutil.Wrap(err)
		}

		return s, nil
	}

	if len(socket) > 0 {
		s, err := socketsource.New(socket, builder, announcer)
		if err != nil {
			return nil, errorutil.Wrap(err)
		}

		return s, nil
	}

	errorutil.Dief(verbose, nil, "No logs sources specified or import flag provided! Use -help to more info.")

	return nil, nil
}
