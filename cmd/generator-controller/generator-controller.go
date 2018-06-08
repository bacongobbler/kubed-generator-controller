package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/bacongobbler/draft-generator-controller/pkg/pack"
)

type generateCmd struct {
	stdout         io.Writer
	pack           string
	dest           string
	repositoryName string
}

var (
	flagDebug   bool
	globalUsage = `Generates boilerplate code that is necessary to write a microservice.

By default it scaffolds your application using the javascript pack, but it can be changed using the --pack flag.
See 'draft generate controller --help' to see what packs are available.
`
)

func newRootCmd(stdout io.Writer, stdin io.Reader, stderr io.Writer) *cobra.Command {
	c := generateCmd{
		stdout: stdout,
	}

	cmd := &cobra.Command{
		Use:          "generator-controller <name>",
		Short:        "generates controllers",
		Long:         globalUsage,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if flagDebug {
				log.SetLevel(log.DebugLevel)
			}
			c.dest = args[0]
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.run()
		},
	}

	f := cmd.Flags()
	f.StringVarP(&c.pack, "pack", "p", "nodejs", "the named starter pack to scaffold the controller with. Default starter packs: [clojure dotnet go maven nodejs php python ruby rust swift]")

	pf := cmd.PersistentFlags()
	pf.BoolVar(&flagDebug, "debug", false, "enable verbose output")

	return cmd
}

func (c *generateCmd) run() error {

	_, err := os.Stat(c.dest)
	log.Debugf("value of err for os.Stat(%s): %v", c.dest, err)
	if os.IsNotExist(err) {
		if err := os.Mkdir(c.dest, 0777); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("there was an error checking if %s exists: %v", c.dest, err)
	}

	// --pack was explicitly defined, so we can just lazily use that here. No detection required.
	// if DRAFT_PLUGIN_DIR is unset, we just fallback to ./packs
	packsFound, err := pack.Find(filepath.Join(os.Getenv("DRAFT_PLUGIN_DIR"), "packs"), c.pack)
	if err != nil {
		return err
	}
	log.Debugf("packs found: %v", packsFound)
	if len(packsFound) == 0 {
		return fmt.Errorf("No packs found with name %s", c.pack)

	} else if len(packsFound) == 1 {
		packSrc := packsFound[0]
		if err = pack.CreateFrom(c.dest, packSrc); err != nil {
			return err
		}

	} else {
		return fmt.Errorf("Multiple packs named %s found: %v", c.pack, packsFound)
	}

	// Each pack makes the assumption that they're listening on port 8080
	addRoute("config/routes", fmt.Sprintf("/%s\t%s\t8080\t/", c.dest, c.dest))

	// TODO: add a Deployment and Service to charts/appname/templates/
	// TODO: append helper functions to charts/appname/templates/_helpers.tpl

	fmt.Fprintln(c.stdout, "--> Ready to sail")
	return nil
}

// addRoute adds a new route to fpath. It appends the route
// above the default route so that it takes higher priority
// in the list than the static files, but lower priority than
// other routes higher up in the list.
func addRoute(fpath, route string) error {
	const defaultRoute = "/\tstatic\t"
	b, err := ioutil.ReadFile(fpath)
	if err != nil {
		return err
	}
	content := string(b)
	fileContent := ""
	n, defaultRouteExists := containsDefaultRoute(content)
	if defaultRouteExists {
		for i, line := range strings.Split(content, "\n") {
			if i == n {
				fileContent += route + "\n"
			}
			fileContent += line + "\n"
		}
	} else {
		fileContent = content
		if !strings.HasSuffix(fileContent, "\n") && fileContent != "" {
			fileContent += "\n"
		}
		fileContent += route + "\n"
	}
	return ioutil.WriteFile(fpath, []byte(fileContent), 0644)
}

// containsDefaultRoute determines if the content contains a line starting with
//
// / static 8080 /
//
// if it does, it returns the line number (0-indexed) where the first instance
// of that route is found.
func containsDefaultRoute(content string) (int, bool) {
	for i, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			if fields[0] == "/" && fields[1] == "static" &&
				fields[2] == "8080" && fields[3] == "/" {
				return i, true
			}
		}
	}
	return 0, false
}

func main() {
	cmd := newRootCmd(os.Stdout, os.Stdin, os.Stderr)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
