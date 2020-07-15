package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gitlab.com/lightmeter/controlcenter/api"
	"gitlab.com/lightmeter/controlcenter/auth"
	"gitlab.com/lightmeter/controlcenter/data"
	"gitlab.com/lightmeter/controlcenter/httpauth"
	"gitlab.com/lightmeter/controlcenter/httpsettings"
	"gitlab.com/lightmeter/controlcenter/logeater"
	"gitlab.com/lightmeter/controlcenter/logeater/dirwatcher"
	"gitlab.com/lightmeter/controlcenter/staticdata"
	"gitlab.com/lightmeter/controlcenter/util"
	"gitlab.com/lightmeter/controlcenter/version"
	"gitlab.com/lightmeter/controlcenter/workspace"
)

type watchableFilenames []string

func (this watchableFilenames) String() string {
	return strings.Join(this, ", ")
}

func (this *watchableFilenames) Set(value string) error {
	*this = append(*this, value)
	return nil
}

var (
	filesToWatch       watchableFilenames
	watchFromStdin     bool
	workspaceDirectory string
	importOnly         bool
	showVersion        bool
	dirToWatch         string
	address            string
	verbose            bool
	emailToPasswdReset string
	passwordToReset    string

	timezone *time.Location = time.UTC
	logYear  int
)

func printVersion() {
	fmt.Fprintf(os.Stderr, "Lightmeter ControlCenter %s\n", version.Version)
}

func init() {
	flag.Var(&filesToWatch, "watch", "File to watch (can be used multiple times)")
	flag.BoolVar(&watchFromStdin, "stdin", false, "Read log lines from stdin")
	flag.StringVar(&workspaceDirectory, "workspace", "lightmeter_workspace", "Path to the directory to store all working data")
	flag.BoolVar(&importOnly, "importonly", false,
		"Only import logs from stdin, exiting immediately, without running the full application. Implies -stdin")
	flag.IntVar(&logYear, "what_year_is_it", time.Now().Year(), "Specify the year when the logs start. Defaults to the current year. This option is temporary and will be removed soon. Promise :-)")
	flag.BoolVar(&showVersion, "version", false, "Show Version Information")
	flag.StringVar(&dirToWatch, "watch_dir", "", "Path to the directory where postfix stores its log files, to be watched")
	flag.StringVar(&address, "listen", ":8080", "Network address to listen to")
	flag.BoolVar(&verbose, "verbose", false, "Be Verbose")
	flag.StringVar(&emailToPasswdReset, "email_reset", "", "Reset password for user (implies -password and depends on -workspace)")
	flag.StringVar(&passwordToReset, "password", "", "Password to reset (requires -email_reset)")

	flag.Usage = func() {
		printVersion()
		flag.PrintDefaults()
	}
}

func die(err error, msg ...interface{}) {
	expandError := func(err error) error {
		if e, ok := err.(*util.Error); ok {
			return e.Chain()
		}

		return err
	}

	log.Println(msg...)

	if verbose {
		log.Print(expandError(err))
	}

	os.Exit(1)
}

func performPasswordReset() {
	auth, err := auth.NewAuth(workspaceDirectory, auth.Options{})

	if err != nil {
		die(util.WrapError(err), "Error opening auth database:", err)
	}

	if err := auth.ChangePassword(emailToPasswdReset, passwordToReset); err != nil {
		die(util.WrapError(err), "Error resetting password:", err)
	}

	if err := auth.Close(); err != nil {
		die(util.WrapError(err), "Error closing auth database:", err)
	}

	log.Println("Password for user", emailToPasswdReset, "reset successfully")
}

func main() {
	flag.Parse()

	if showVersion {
		printVersion()
		return
	}

	if len(emailToPasswdReset) > 0 {
		performPasswordReset()
		return
	}

	postfixLogsDirContent := func() dirwatcher.DirectoryContent {
		if len(dirToWatch) != 0 {
			dir, err := dirwatcher.NewDirectoryContent(dirToWatch)

			if err != nil {
				die(util.WrapError(err), "Error opening directory:", dirToWatch)
			}

			return dir
		}

		return nil
	}()

	if postfixLogsDirContent != nil {
		initialLogTimeFromDirectory, err := dirwatcher.FindInitialLogTime(postfixLogsDirContent)

		if err != nil {
			die(util.WrapError(err), "Could not obtain initial log time from directory:", dirToWatch)
		}

		log.Println("Using initial time from postfix log directory:", initialLogTimeFromDirectory)
		logYear = initialLogTimeFromDirectory.Year()
	}

	ws, err := workspace.NewWorkspace(workspaceDirectory, data.Config{
		Location: timezone,
	})

	if err != nil {
		die(util.WrapError(err), "Error opening workspace directory:", workspaceDirectory)
	}

	doneWithDatabase := ws.Run()

	pub := ws.NewPublisher()

	if importOnly {
		parseLogsFromStdin(pub)
		<-doneWithDatabase
		log.Println("Importing has finished. Bye!")
		return
	}

	if watchFromStdin {
		go parseLogsFromStdin(pub)
	}

	logFilesWatchLocation := logeater.FindWatchingLocationForWorkspace(&ws)

	for _, filename := range filesToWatch {
		log.Println("Now watching file", filename, "for changes from the", func() string {
			if logFilesWatchLocation.Whence == os.SEEK_END {
				return "end"
			}

			return "beginning"
		}())

		go func(filename string) {
			if err := logeater.WatchFile(filename, logFilesWatchLocation, pub); err != nil {
				die(util.WrapError(err), "Error watching file:", filename)
			}
		}(filename)
	}

	if postfixLogsDirContent != nil {
		initialTime := func() time.Time {
			t := ws.MostRecentLogTime()

			if t.IsZero() {
				return time.Date(1970, time.January, 1, 0, 0, 0, 0, timezone)
			}

			return t
		}()

		log.Println("Start importing Postfix logs directory from time", initialTime)

		watcher := dirwatcher.NewDirectoryImporter(postfixLogsDirContent, pub, initialTime)

		go func() {
			if err := watcher.Run(); err != nil {
				die(util.WrapError(err), "Error watching directory:", dirToWatch)
			}
		}()
	}

	dashboard, err := ws.Dashboard()

	if err != nil {
		die(util.WrapError(err), "Error creating dashboard")
	}

	settings := ws.Settings()

	initialSetupHandler := httpsettings.NewInitialSetupHandler(settings)

	mux := http.NewServeMux()

	exposeApiExplorer(mux)

	exposeProfiler(mux)

	api.HttpDashboard(mux, timezone, dashboard)

	mux.Handle("/", http.FileServer(staticdata.HttpAssets))

	mux.Handle("/settings/initialSetup", initialSetupHandler)

	// Some paths that don't require authentication
	// That's what people nowadays call a "allow list".
	publicPaths := []string{
		"/img",
		"/css",
		"/fonts",
		"/js",
		"/3rd",
		"/debug",
	}

	authWrapper := httpauth.Serve(mux, ws.Auth(), workspaceDirectory, publicPaths)

	log.Fatal(http.ListenAndServe(address, authWrapper))
}

func parseLogsFromStdin(publisher data.Publisher) {
	logeater.ReadFromReader(os.Stdin, publisher)
	publisher.Close()
	log.Println("STDIN has just closed!")
}
