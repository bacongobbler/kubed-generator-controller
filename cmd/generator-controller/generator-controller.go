package main

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/Azure/draft/pkg/draft/manifest"
	"github.com/BurntSushi/toml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/bacongobbler/draft-generator-controller/pkg/pack"
)

const (
	environmentEnvVar = "DRAFT_ENV"
	globalUsage       = `Generates boilerplate code that is necessary to write a microservice.

By default it scaffolds your application using the javascript pack, but it can be changed using the --pack flag.
See 'draft generate controller --help' to see what packs are available.
`
	deploymentTemplate = `kind: Deployment
apiVersion: apps/v1
metadata:
  name: {{ template "{% .AppName %}.{% .Name %}.name" . }}
  labels:
    draft: {{ template "{% .AppName %}.name" . }}
    controller: {% .Name %}
spec:
  selector:
    matchLabels:
      draft: {{ template "{% .AppName %}.name" . }}
      controller: {% .Name %}
  replicas: {{ default .Values.{% .Name %}.replicaCount 1 }}
  template:
    metadata:
      annotations:
        buildID: {{ .Values.buildID }}
      labels:
        draft: {{ template "{% .AppName %}.name" . }}
        controller: {% .Name %}
    spec:
      containers:
        - name: {% .Name %}
          image: "{{ .Values.{% .Name %}.image.repository }}:{{ .Values.{% .Name %}.image.tag }}"
          imagePullPolicy: {{ default .Values.{% .Name %}.image.pullPolicy "IfNotPresent" }}
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
`
	serviceTemplate = `kind: Service
apiVersion: v1
metadata:
  name: {{ template "{% .AppName %}.{% .Name %}.name" . }}
  labels:
    draft: {{ template "{% .AppName %}.name" . }}
    controller: {% .Name %}
spec:
  selector:
    draft: {{ template "{% .AppName %}.name" . }}
    controller: {% .Name %}
  ports:
    - port: 80
      targetPort: http
      protocol: TCP
      name: http
`
	helperTemplate = `
{{- define "{% .AppName %}.{% .Name %}.name" -}}
{{- printf "%s-{% .Name %}" .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
`
	valuesTemplate = `
{% .Name %}:
  image: {}
`
)

var flagDebug bool

type generateCmd struct {
	stdout         io.Writer
	pack           string
	name           string
	repositoryName string
}

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
			c.name = args[0]
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
	// --pack was explicitly defined, so we can just lazily use that here. No detection required.
	// if DRAFT_PLUGIN_DIR is unset, we just fallback to ./packs
	packsFound, err := pack.Find(filepath.Join(os.Getenv("DRAFT_PLUGIN_DIR"), "packs"), c.pack)
	if err != nil {
		return err
	}

	var draftConfig manifest.Manifest
	if _, err := toml.DecodeFile(filepath.Join("config", "draft.toml"), &draftConfig); err != nil {
		return err
	}
	appConfig, found := draftConfig.Environments[defaultDraftEnvironment()]
	if !found {
		return fmt.Errorf("Environment %v not found", defaultDraftEnvironment())
	}

	deploymentFile, err := os.Create(filepath.Join("charts", appConfig.Name, "templates", fmt.Sprintf("%s-deployment.yaml", c.name)))
	if err != nil {
		return err
	}
	defer deploymentFile.Close()
	serviceFile, err := os.Create(filepath.Join("charts", appConfig.Name, "templates", fmt.Sprintf("%s-service.yaml", c.name)))
	if err != nil {
		return err
	}
	defer serviceFile.Close()
	valuesFile, err := os.OpenFile(filepath.Join("charts", appConfig.Name, "values.yaml"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer valuesFile.Close()
	helpersFile, err := os.OpenFile(filepath.Join("charts", appConfig.Name, "templates", "_helpers.tpl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer helpersFile.Close()

	// scaffold helm chart
	values := struct {
		AppName string
		Name    string
	}{
		AppName: appConfig.Name,
		Name:    c.name,
	}
	dt := template.Must(template.New("deployment").Delims("{%", "%}").Parse(deploymentTemplate))
	if err := dt.Execute(deploymentFile, values); err != nil {
		return err
	}

	st := template.Must(template.New("service").Delims("{%", "%}").Parse(serviceTemplate))
	if err := st.Execute(serviceFile, values); err != nil {
		return err
	}

	vt := template.Must(template.New("values").Delims("{%", "%}").Parse(valuesTemplate))
	if err := vt.Execute(valuesFile, values); err != nil {
		return err
	}

	ht := template.Must(template.New("helpers").Delims("{%", "%}").Parse(helperTemplate))
	if err := ht.Execute(helpersFile, values); err != nil {
		return err
	}

	// scaffold business logic
	if _, err := os.Stat(c.name); os.IsNotExist(err) {
		if err := os.Mkdir(c.name, 0777); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("there was an error checking if %s exists: %v", c.name, err)
	}
	log.Debugf("packs found: %v", packsFound)
	if len(packsFound) == 0 {
		return fmt.Errorf("No packs found with name %s", c.pack)

	} else if len(packsFound) == 1 {
		packSrc := packsFound[0]
		if err = pack.CreateFrom(c.name, packSrc); err != nil {
			return err
		}

	} else {
		return fmt.Errorf("Multiple packs named %s found: %v", c.pack, packsFound)
	}

	// Each pack makes the assumption that they're listening on port 8080
	addRoute(filepath.Join("config", "routes"), fmt.Sprintf("/%s\t%s\t8080\t/", c.name, c.name))

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

func defaultDraftEnvironment() string {
	env := os.Getenv(environmentEnvVar)
	if env == "" {
		env = manifest.DefaultEnvironmentName
	}
	return env
}
