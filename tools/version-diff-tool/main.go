package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/mod/semver"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/tufin/oasdiff/checker"
	"github.com/tufin/oasdiff/diff"
	"github.com/tufin/oasdiff/load"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

type CRD struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

type Metadata struct {
	Name string `yaml:"name"`
}

type Spec struct {
	Names    Names     `yaml:"names"`
	Versions []Version `yaml:"versions"`
}

type Names struct {
	Kind string `yaml:"kind"`
}

type Version struct {
	Schema struct {
		OpenAPIV3Schema OpenAPIV3Schema `yaml:"openAPIV3Schema"`
	} `yaml:"schema"`
}

type OpenAPIV3Schema struct {
	Properties map[string]Property `yaml:"properties"`
}

type Property struct {
	Description string                 `yaml:"description,omitempty"`
	Properties  map[string]interface{} `yaml:"properties"`
	Required    []string               `yaml:"required,omitempty"`
	Type        string                 `yaml:"type"`
}

type Content struct {
	Schema Schema `yaml:"schema"`
}

type Schema struct {
	Description string                 `yaml:"description"`
	Type        string                 `yaml:"type"`
	Required    []string               `yaml:"required"`
	Properties  map[string]interface{} `yaml:"properties"`
}

type Path struct {
	Post Post `yaml:"post"`
}

type Post struct {
	RequestBody RequestBody `yaml:"requestBody"`
}

type RequestBody struct {
	Required bool               `yaml:"required"`
	Content  map[string]Content `yaml:"content"`
}

func createOpenAPISpec(crds []CRD) map[string]interface{} {
	paths := make(map[string]interface{})
	for _, crd := range crds {
		path := fmt.Sprintf("%s.%s", strings.ToLower(crd.Spec.Names.Kind), "hazelcast.com")
		schema := Schema{
			Description: crd.Metadata.Name,
			Type:        "object",
			Required:    []string{"spec"},
			Properties: map[string]interface{}{
				"apiVersion": map[string]interface{}{"type": "string"},
				"kind":       map[string]interface{}{"type": "string"},
				"metadata":   map[string]interface{}{"type": "object"},
				"spec":       crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"],
				"status":     crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["status"],
			},
		}

		paths[path] = Path{
			Post: Post{
				RequestBody: RequestBody{
					Required: true,
					Content: map[string]Content{
						"application/json": {
							Schema: schema,
						},
					},
				},
			},
		}
	}
	return map[string]interface{}{"paths": paths}
}

func addRepo(settings *cli.EnvSettings, repoName, repoURL string) error {
	repoEntry := &repo.Entry{
		Name: repoName,
		URL:  repoURL,
	}
	chartRepo, err := repo.NewChartRepository(repoEntry, getter.All(settings))
	if err != nil {
		return fmt.Errorf("failed to create chart repository: %v", err)
	}
	if _, err := chartRepo.DownloadIndexFile(); err != nil {
		return fmt.Errorf("failed to download index file: %v", err)
	}
	return nil
}

func generateCRDFile(version, repoUrl string) (string, error) {
	settings := cli.New()
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		return "", fmt.Errorf("failed to initialize Helm action configuration: %v", err)
	}

	const repoName = "hazelcast"
	const crdChartName = "hazelcast-platform-operator-crds"
	fullChartName := repoUrl
	if strings.HasPrefix(repoUrl, "https://") {
		fullChartName = fmt.Sprintf("%s/%s", repoName, crdChartName)
		if err := addRepo(settings, repoName, repoUrl); err != nil {
			return "", err
		}
	}
	client := action.NewInstall(actionConfig)
	client.DryRun = true
	client.ReleaseName = repoName
	client.Replace = true
	client.ClientOnly = true
	client.IncludeCRDs = true
	client.Version = version

	chartPath, err := client.ChartPathOptions.LocateChart(fullChartName, settings)
	if err != nil {
		return "", fmt.Errorf("failed to locate chart: %v", err)
	}

	chart, err := loader.Load(chartPath)
	if err != nil {
		return "", fmt.Errorf("failed to load chart: %v", err)
	}

	release, err := client.Run(chart, nil)
	if err != nil {
		return "", fmt.Errorf("failed to render chart: %v", err)
	}

	outputFile := fmt.Sprintf("%s.yaml", version)
	if err := os.WriteFile(outputFile, []byte(release.Manifest), 0644); err != nil {
		return "", fmt.Errorf("failed to write CRD file to %s: %v", outputFile, err)
	}
	return outputFile, nil
}

func extractCRDs(inputFile string) ([]CRD, error) {
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	var crds []CRD
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	for {
		var crd CRD
		if err := decoder.Decode(&crd); err != nil {
			break
		}
		if crd.Kind == "CustomResourceDefinition" {
			crds = append(crds, crd)
		}
	}
	return crds, nil
}

func writeOpenAPISpec(outputFile string, openAPISpec map[string]interface{}) error {
	data, err := yaml.Marshal(&openAPISpec)
	if err != nil {
		return fmt.Errorf("failed to marshal OpenAPI spec: %v", err)
	}
	return os.WriteFile(outputFile, data, 0644)
}

func formatOutput(output string) string {
	output = regexp.MustCompile(`This is a warning because.*?change in specification\.`).ReplaceAllString(output, "")
	output = regexp.MustCompile(`in API`).ReplaceAllString(output, "")
	output = regexp.MustCompile(`POST`).ReplaceAllString(output, "in")
	output = strings.Replace(output, "/", ".", 6)
	return output
}

func ignoreChanges(output string, ignoreMessages []string) string {
	blocks := strings.Split(output, "\n\n")
	var filteredBlocks []string

blockLoop:
	for _, block := range blocks {
		for _, message := range ignoreMessages {
			if strings.Contains(block, message) {
				continue blockLoop
			}
		}
		filteredBlocks = append(filteredBlocks, block)
	}
	return strings.Join(filteredBlocks, "\n\n")
}

func generateAndExtractCRDs(version, outputFile, repoURL string, wg *sync.WaitGroup, specInfo **load.SpecInfo, apiLoader *openapi3.Loader) {
	defer wg.Done()
	rawCrdFile, err := generateCRDFile(version, repoURL)
	if err != nil {
		log.Fatalf("failed to generate CRD file for version %s and repo %s: %v", version, repoURL, err)
	}

	crds, err := extractCRDs(rawCrdFile)
	if err != nil {
		log.Fatalf("failed to extract CRDs from file for version %s: %v", version, err)
	}

	openAPISpec := createOpenAPISpec(crds)
	if err := writeOpenAPISpec(outputFile, openAPISpec); err != nil {
		log.Fatalf("failed to write OpenAPI spec for version %s: %v", version, err)
	}

	*specInfo, err = load.NewSpecInfo(apiLoader, load.NewSource(outputFile))
	if err != nil {
		log.Fatalf("failed to load spec info for version %s: %v", version, err)
	}
}

var ignoreMessagesList = []string{
	`the request property 'spec.licenseKeySecretName' became required`,
	`the 'spec.licenseKeySecretName' request property's minLength was increased from '0' to '1'`,
	`the request property 'spec.licenseKeySecretName' became required`,
	`the 'spec.licenseKeySecretName' request property's minLength was increased from '0' to '1'`,
}

func main() {
	base := flag.String("base", "", "Version of the first CRD to compare")
	revision := flag.String("revision", "", "Version of the second CRD to compare")
	ignoreMessages := flag.String("ignore", "", "Comma-separated list of messages https://github.com/Tufin/oasdiff/blob/main/checker/localizations_src/en/messages.yaml to ignore in the output")
	baseRepoURL := flag.String("base-repo-url", "https://hazelcast-charts.s3.amazonaws.com", "URL of the base Helm chart repository")
	revisionRepoURL := flag.String("revision-repo-url", "https://hazelcast-charts.s3.amazonaws.com", "URL of the revision Helm chart repository")
	flag.Parse()

	if *base == "" && *revision == "" {
		log.Fatal("both versions (-base, -revision) must be provided")
	}

	if *base != "" && semver.Compare(fmt.Sprintf("v%s", *base), "v5.5") <= 0 {
		log.Fatalf("base version %s is not supported for backward compatibility check. Versions must be greater than 5.5. Starting from version 5.6, Helm charts are used for setup instead of bundle files. This tool supports only Helm chart setup.", *base)
	}

	if *revision != "" && *revision != "latest-snapshot" && semver.Compare(fmt.Sprintf("v%s", *revision), "v5.5") <= 0 {
		log.Fatalf("revision version %s is not supported for backward compatibility check. Versions must be greater than 5.5 unless it is 'latest-snapshot'. Starting from version 5.6, Helm charts are used for setup instead of bundle files. This tool supports only Helm chart setup.", *revision)
	}

	var ignoreList = ignoreMessagesList
	if *ignoreMessages != "" {
		ignoreList = strings.Split(*ignoreMessages, ",")
	}

	apiLoader := openapi3.NewLoader()
	apiLoader.IsExternalRefsAllowed = true
	var wg sync.WaitGroup
	var baseSpec, revisionSpec *load.SpecInfo
	baseFile := fmt.Sprintf("%s.yaml", *base)
	revisionFile := fmt.Sprintf("%s.yaml", *revision)

	wg.Add(2)

	go generateAndExtractCRDs(*base, baseFile, *baseRepoURL, &wg, &baseSpec, apiLoader)
	go generateAndExtractCRDs(*revision, revisionFile, *revisionRepoURL, &wg, &revisionSpec, apiLoader)

	wg.Wait()

	diffRes, operationsSources, err := diff.GetPathsDiff(diff.NewConfig(), []*load.SpecInfo{baseSpec}, []*load.SpecInfo{revisionSpec})
	if err != nil {
		log.Fatalf("diff failed with %v", err)
	}

	errs := checker.CheckBackwardCompatibility(checker.GetDefaultChecks(), diffRes, operationsSources)
	if len(errs) >= 0 {
		var formattedOutput string
		localization := checker.NewDefaultLocalizer()
		infoMessage := fmt.Sprintf("Comparing CRD files: %s and %s\n", *base, *revision)
		for _, bc := range errs {
			output := bc.MultiLineError(localization, checker.ColorAlways)
			formattedOutput += fmt.Sprintf("\n%s\n", formatOutput(output))
		}
		filteredOutput := ignoreChanges(formattedOutput, ignoreList)
		errorCount := strings.Count(filteredOutput, "error")
		warningCount := strings.Count(filteredOutput, "warning")
		summary := fmt.Sprintf("%s\n%d breaking changes: %d error, %d warning\n", infoMessage, errorCount+warningCount, errorCount, warningCount)
		fmt.Printf("%s%s", summary, filteredOutput)
	}
}
