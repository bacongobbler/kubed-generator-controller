package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/Azure/draft/pkg/draft/draftpath"
	"github.com/Azure/draft/pkg/draft/pack"
	"github.com/Azure/draft/pkg/draft/pack/repo"
	"github.com/Azure/draft/pkg/linguist"
	"github.com/Azure/draft/pkg/osutil"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type generateCmd struct {
	stdout         io.Writer
	pack           string
	home           draftpath.Home
	dest           string
	repositoryName string
}

// ErrNoLanguageDetected is raised when `draft create` does not detect source
// code for linguist to classify, or if there are no packs available for the detected languages.
var ErrNoLanguageDetected = errors.New("no languages were detected")

var (
	flagDebug   bool
	globalUsage = `The controller generator for Draft.
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
			c.home = draftpath.Home(os.Getenv("DRAFT_HOME"))
			c.dest = args[0]
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.run()
		},
	}

	f := cmd.Flags()
	f.StringVarP(&c.pack, "pack", "p", "", "the named Draft starter pack to scaffold the controller with")

	pf := cmd.PersistentFlags()
	pf.BoolVar(&flagDebug, "debug", false, "enable verbose output")

	return cmd
}

func (c *generateCmd) run() error {

	appExists, err := osutil.Exists(c.dest)
	if err != nil {
		return fmt.Errorf("there was an error checking if %s exists: %v", c.dest, err)
	}
	if !appExists {
		if err := os.Mkdir(c.dest, 0777); err != nil {
			return err
		}
	}

	if c.pack != "" {
		// --pack was explicitly defined, so we can just lazily use that here. No detection required.
		packsFound, err := pack.Find(c.home.Packs(), c.pack)
		if err != nil {
			return err
		}
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

	} else {
		// pack detection time
		packPath, err := c.doPackDetection()
		if err != nil {
			return err
		}
		err = pack.CreateFrom(c.dest, packPath)
		if err != nil {
			return err
		}
	}

	fmt.Fprintln(c.stdout, "--> Ready to sail")
	return nil
}

func (c *generateCmd) normalizeApplicationName() {
	if c.dest == "" {
		return
	}

	nameIsUpperCase := false
	for _, char := range c.dest {
		if unicode.IsUpper(char) {
			nameIsUpperCase = true
			break
		}
	}

	if !nameIsUpperCase {
		return
	}

	normalized := strings.ToLower(c.dest)
	normalized = strings.Replace(normalized, "/", "-", -1)
	normalized = strings.Replace(normalized, "\\", "-", -1)
	fmt.Fprintf(
		c.stdout,
		"--> Application %s will be renamed to %s for docker compatibility\n",
		c.dest,
		normalized,
	)
	c.dest = normalized
}

// doPackDetection performs pack detection across all the packs available in $(draft home)/packs in
// alphabetical order, returning the pack dirpath and any errors that occurred during the pack detection.
func (c *generateCmd) doPackDetection() (string, error) {
	langs, err := linguist.ProcessDir(c.dest)
	log.Debugf("linguist.ProcessDir('.') result:\n\nError: %v", err)
	if err != nil {
		return "", fmt.Errorf("there was an error detecting the language: %s", err)
	}
	for _, lang := range langs {
		log.Debugf("%s:\t%f (%s)", lang.Language, lang.Percent, lang.Color)
	}
	if len(langs) == 0 {
		return "", ErrNoLanguageDetected
	}
	for _, lang := range langs {
		detectedLang := linguist.Alias(lang)
		fmt.Fprintf(c.stdout, "--> Draft detected %s (%f%%)\n", detectedLang.Language, detectedLang.Percent)
		for _, repository := range repo.FindRepositories(c.home.Packs()) {
			packDir := filepath.Join(repository.Dir, repo.PackDirName)
			packs, err := ioutil.ReadDir(packDir)
			if err != nil {
				return "", fmt.Errorf("there was an error reading %s: %v", packDir, err)
			}
			for _, file := range packs {
				if file.IsDir() {
					if strings.Compare(strings.ToLower(detectedLang.Language), strings.ToLower(file.Name())) == 0 {
						packPath := filepath.Join(packDir, file.Name())
						log.Debugf("pack path: %s", packPath)
						return packPath, nil
					}
				}
			}
		}
		fmt.Fprintf(c.stdout, "--> Could not find a pack for %s. Trying to find the next likely language match...\n", detectedLang.Language)
	}
	return "", ErrNoLanguageDetected
}

func main() {
	cmd := newRootCmd(os.Stdout, os.Stdin, os.Stderr)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
