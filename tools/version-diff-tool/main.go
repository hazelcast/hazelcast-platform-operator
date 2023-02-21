package main

import (
	"fmt"
	"github.com/tidwall/sjson"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sigs.k8s.io/yaml"
	"strings"
)

const (
	crdNamesSelector   = `yq '.. | select(has("singular")).singular' %s`
	schemaSelector     = `yq '.. | select(has("schema")).schema' %s`
	descriptionDeleter = `yq 'del(.. | select(has("description")).description)' %s > r%s`
	openAPIYAML        = `openapi: 3.0.0
info:
  version: 1.0.0
  title: Sample API
  description: A sample API to illustrate OpenAPI concepts
components:
  schemas:
`
	diff    = `oasdiff -base %s -revision %s > kdiff.yaml`
	flatten = `yq eval '.. | select((tag == "!!map" or tag == "!!seq") | not) | (path | join(".")) + "=" + .' kdiff.yaml`
)

func main() {

	cmd := exec.Command("git", "for-each-ref", `--format='%(refname)'`, "refs/tags")
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic(err)
	}

	tags := findTags(string(out))
	yamlFileNames, err := downloadBundleYAMLs(tags)
	if err != nil {
		panic(err)
	}

	yamlFileNames = []string{"bundle-5.3.yaml", "bundle-5.4.yaml"}
	_, err = createOpenAPISpec(yamlFileNames)
	if err != nil {
		panic(err)
	}

	err = compareSpecs("open-api-bundle-5.3.yaml", "open-api-bundle-5.4.yaml")
	if err != nil {
		panic(err)
	}

	err = formatDiff()
	if err != nil {
		panic(err)
	}
}

func findTags(tags string) []string {
	tags = strings.ReplaceAll(tags, "'", "")
	tags = strings.ReplaceAll(tags, "refs/tags/v", "")
	tagList := strings.Split(tags, "\n")

	var detectedTags []string
	for i := len(tagList) - 1; i > 0; i-- {
		if tagList[i] != "" {
			detectedTags = append(detectedTags, tagList[i])
		}
		if len(detectedTags) == 2 {
			break
		}
	}

	return detectedTags
}

func downloadBundleYAMLs(tags []string) ([]string, error) {
	var fileNames []string

	for _, t := range tags {
		//TODO: semantic version comparison (> 5.5)
		// https://repository.hazelcast.com/operator/bundle-5.5.yaml
		// helm install operator hazelcast/hazelcast-platform-operator --dry-run --debug --set installCRDs=true > 5.6.yaml

		resp, err := http.Get(fmt.Sprintf("https://repository.hazelcast.com/operator/bundle-%s.yaml", t))
		if err != nil {
			return []string{}, err
		}

		f, err := os.Create(fmt.Sprintf("bundle-%s.yaml", t))
		if err != nil {
			return []string{}, err
		}

		_, err = io.Copy(f, resp.Body)
		if err != nil {
			return []string{}, err
		}

		fileNames = append(fileNames, f.Name())
	}

	return fileNames, nil
}

type parsedYAML struct {
	fileName string
	crdNames []string
	schemas  []string
}

func createOpenAPISpec(yamlFiles []string) ([]string, error) {
	var yamls []parsedYAML

	for _, y := range yamlFiles {
		cmd := exec.Command("bash", "-c", fmt.Sprintf(descriptionDeleter, y, y))
		_, err := cmd.CombinedOutput()
		if err != nil {
			return []string{}, err
		}

		descriptionLessFileName := "r" + y

		cmd = exec.Command("bash", "-c", fmt.Sprintf(crdNamesSelector, descriptionLessFileName))
		o2, err := cmd.CombinedOutput()
		if err != nil {
			return []string{}, err
		}

		cmd = exec.Command("bash", "-c", fmt.Sprintf(schemaSelector, descriptionLessFileName))
		o3, err := cmd.CombinedOutput()
		if err != nil {
			return []string{}, err
		}

		p := parsedYAML{
			fileName: y,
			crdNames: strings.Split(string(o2), "---"),
			schemas:  strings.Split(string(o3), "---"),
		}

		// clean up some fields to make them the same format
		for i := range p.crdNames {
			p.crdNames[i] = strings.ReplaceAll(p.crdNames[i], "\n", "")
		}

		for i := range p.schemas {
			p.schemas[i] = strings.ReplaceAll(p.schemas[i], "openAPIV3Schema:\n", "")
		}

		yamls = append(yamls, p)
	}

	mergeCRDNamesAndSchemas(yamls)

	return []string{}, nil
}

func mergeCRDNamesAndSchemas(yamls []parsedYAML) error {
	specs := make(map[string][]byte) // fileName -> openAPISpec YAML

	for _, y := range yamls {
		var wholeSchema []byte
		for i := range y.schemas {
			s, err := yaml.YAMLToJSON([]byte(y.schemas[i]))
			if err != nil {
				return err
			}

			wholeSchema, err = sjson.SetRawBytes(wholeSchema, y.crdNames[i], s)
			if err != nil {
				return err
			}
		}

		specs[y.fileName] = wholeSchema
	}

	j1, err := yaml.YAMLToJSON([]byte(openAPIYAML))
	if err != nil {
		return err
	}

	json1, err := sjson.SetRawBytes(j1, "components.schemas", specs[yamls[0].fileName])
	if err != nil {
		return err
	}

	yaml1, err := yaml.JSONToYAML(json1)
	if err != nil {
		return err
	}

	file1, err := os.Create("open-api-" + yamls[0].fileName)
	if err != nil {
		return err
	}
	defer file1.Close()
	_, err = file1.WriteString(string(yaml1))
	if err != nil {
		return err
	}

	j2, err := yaml.YAMLToJSON([]byte(openAPIYAML))
	if err != nil {
		return err
	}

	json2, err := sjson.SetRawBytes(j2, "components.schemas", specs[yamls[1].fileName])
	if err != nil {
		return err
	}

	yaml2, err := yaml.JSONToYAML(json2)
	if err != nil {
		return err
	}

	file2, err := os.Create("open-api-" + yamls[1].fileName)
	if err != nil {
		return err
	}
	defer file2.Close()
	_, err = file2.WriteString(string(yaml2))
	if err != nil {
		return err
	}

	return nil
}

func compareSpecs(file1, file2 string) error {
	cmd := exec.Command("bash", "-c", fmt.Sprintf(diff, file1, file2))
	_, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}
	return nil
}

func formatDiff() error {
	cmd := exec.Command("bash", "-c", flatten)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	fmt.Println(output)

	return nil
}
