package main

import (
	"flag"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/tidwall/sjson"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"strings"
)

const (
	// File names
	outputFileName          = `diff_%s_%s.txt`
	descriptionlessFileName = `r%s`
	bundleFileName          = `bundle-%s.yaml`
	openAPIFileName         = `open-api-%s`

	// Splitters
	schemaSplitter = `---`

	// Links
	bundleDownloadLink = `https://repository.hazelcast.com/operator/` + bundleFileName

	// Helm related
	helmInstall      = `helm template operator hazelcast/hazelcast-platform-operator-crds --version=%s > ` + bundleFileName
	helmChartVersion = "5.6"

	// yq commands
	crdNamesSelector   = `yq '.. | select(has("singular")).singular' %s`
	schemaSelector     = `yq '.. | select(has("schema")).schema' %s`
	descriptionDeleter = `yq 'del(.. | select(has("description")).description)' %s > ` + descriptionlessFileName
	flatten            = `yq eval '.. | select((tag == "!!map" or tag == "!!seq") | not) | (path | join(".")) + "=" + .' kdiff.yaml | sed 's/components.schemas.//g' | sed 's/properties.//g' | sed 's/modified.//g' | sed 's/stringsdiff.//g'`

	// oasdiff commands
	diff = `oasdiff -base %s -revision %s > kdiff.yaml`

	// Open API YAML
	openAPIYAML = `openapi: 3.0.0
info:
  version: 1.0.0
  title: Sample API
  description: A sample API to illustrate OpenAPI concepts
components:
  schemas:
`
)

func main() {
	base, revision, err := parseParams()
	if err != nil {
		panic(fmt.Errorf("an error occurred while parsing parameters: %s", err.Error()))
	}

	baseBundle, err := downloadBundleYAML(base)
	if err != nil {
		panic(fmt.Errorf("an error occurred while downloading base bundle yaml: %s", err.Error()))
	}

	revBundle, err := downloadBundleYAML(revision)
	if err != nil {
		panic(fmt.Errorf("an error occurred while downloading revision bundle yaml: %s", err.Error()))
	}

	baseSpec, err := createOpenAPISpec(baseBundle)
	if err != nil {
		panic(fmt.Errorf("an error occurred while creating base api spec: %s", err.Error()))
	}

	revSpec, err := createOpenAPISpec(revBundle)
	if err != nil {
		panic(fmt.Errorf("an error occurred while creating revision api spec: %s", err.Error()))
	}

	diffFile, err := compareSpecs(baseSpec, revSpec)
	if err != nil {
		panic(fmt.Errorf("an error occurred while comparing specs: %s", err.Error()))
	}

	err = cleanup()
	if err != nil {
		panic(fmt.Errorf("an error occurred while cleaning up: %s", err.Error()))
	}

	fmt.Println("Spec diff is extracted to file: " + diffFile)
}

func parseParams() (string, string, error) {
	var base, revision string

	flag.StringVar(&base, "base", "", "base yaml")
	flag.StringVar(&revision, "revision", "", "revision yaml")
	flag.Parse()

	if base != "" && revision != "" {
		_, err := version.NewVersion(base)
		if err != nil {
			return "", "", err
		}
		_, err = version.NewVersion(revision)
		if err != nil {
			return "", "", err
		}
	} else {
		cmd := exec.Command("git", "for-each-ref", `--format='%(refname)'`, "refs/tags")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", "", err
		}

		base, revision = findTags(string(out))
	}

	return base, revision, nil
}

func findTags(tags string) (string, string) {
	tags = strings.ReplaceAll(tags, "'", "")
	tags = strings.ReplaceAll(tags, "refs/tags/v", "")
	tagList := strings.Split(tags, "\n")

	revision := tagList[len(tagList)-1]
	base := tagList[len(tagList)-2]

	return base, revision
}

func downloadBundleYAML(v string) (string, error) {
	helmVersion, _ := version.NewVersion(helmChartVersion)
	vv, _ := version.NewVersion(v) // versions already validated, so ignore error

	if vv.GreaterThanOrEqual(helmVersion) {
		cmd := exec.Command("bash", "-c", fmt.Sprintf(helmInstall, v, v))
		_, err := cmd.CombinedOutput()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf(bundleFileName, v), nil
	}

	resp, err := http.Get(fmt.Sprintf(bundleDownloadLink, v))
	if err != nil {
		return "", err
	}

	f, err := os.Create(fmt.Sprintf(bundleFileName, v))
	if err != nil {
		return "", err
	}

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return "", err
	}

	return f.Name(), nil
}

func createOpenAPISpec(yaml string) (string, error) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf(descriptionDeleter, yaml, yaml))
	_, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	cmd = exec.Command("bash", "-c", fmt.Sprintf(crdNamesSelector, fmt.Sprintf(descriptionlessFileName, yaml)))
	o1, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	cmd = exec.Command("bash", "-c", fmt.Sprintf(schemaSelector, fmt.Sprintf(descriptionlessFileName, yaml)))
	o2, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	crdNames := strings.Split(string(o1), schemaSplitter)
	for i := range crdNames {
		crdNames[i] = strings.ReplaceAll(crdNames[i], "\n", "")
	}

	schemas := strings.Split(string(o2), schemaSplitter)
	for i := range schemas {
		schemas[i] = strings.ReplaceAll(schemas[i], "openAPIV3Schema:\n", "")
	}

	n, err := mergeCRDNamesAndSchemas(yaml, crdNames, schemas)
	if err != nil {
		return "", err
	}

	return n, nil
}

func mergeCRDNamesAndSchemas(fileName string, crdNames, schemas []string) (string, error) {
	var wholeSchema []byte
	for i := range schemas {
		s, err := yaml.YAMLToJSON([]byte(schemas[i]))
		if err != nil {
			return "", err
		}

		wholeSchema, err = sjson.SetRawBytes(wholeSchema, crdNames[i], s)
		if err != nil {
			return "", err
		}
	}

	j, err := yaml.YAMLToJSON([]byte(openAPIYAML))
	if err != nil {
		return "", err
	}

	js, err := sjson.SetRawBytes(j, "components.schemas", wholeSchema)
	if err != nil {
		return "", err
	}

	y, err := yaml.JSONToYAML(js)
	if err != nil {
		return "", err
	}

	f, err := os.Create(fmt.Sprintf(openAPIFileName, fileName))
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = f.WriteString(string(y))
	if err != nil {
		return "", err
	}

	return f.Name(), nil
}

func compareSpecs(base, rev string) (string, error) {
	cmd := exec.Command("bash", "-c", fmt.Sprintf(diff, base, rev))
	_, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	diffFile, err := formatDiff(base, rev)
	if err != nil {
		return "", err
	}
	return diffFile, nil
}

func formatDiff(base, rev string) (string, error) {
	cmd := exec.Command("bash", "-c", flatten)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	f, err := os.Create(fmt.Sprintf(outputFileName, base, rev))
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = f.WriteString(string(output))
	if err != nil {
		return "", err
	}

	return f.Name(), nil
}

func cleanup() error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	err = filepath.Walk(wd, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".yaml" {
			os.Remove(info.Name())
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
