package main

import (
	"bytes"
	"flag"
	"fmt"
	sprig "github.com/go-task/slim-sprig"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"strings"
	"text/template"
)

type Rule struct {
	APIGroups []string `yaml:"apiGroups"`
	Resources []string `yaml:"resources"`
	Verbs     []string `yaml:"verbs"`
}

type Metadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace,omitempty"`
}

type Role struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Rules      []Rule   `yaml:"rules"`
}

var tpl *template.Template

func main() {
	helpersTplPath := flag.String("helpers-tpl-path", "../../helm-charts/hazelcast-platform-operator/templates/_helpers.tpl", "Path to the helpers template file")
	rolePath := flag.String("role-path", "../../config/rbac/role.yaml", "Path to the input role YAML file")
	flag.Parse()

	var err error
	tpl, err = template.New("helpers").Funcs(sprig.TxtFuncMap()).Funcs(template.FuncMap{
		"include": func(name string, data interface{}) (string, error) {
			return "", nil
		},
	}).ParseFiles(*helpersTplPath)
	if err != nil {
		panic(fmt.Sprintf("failed to parse templates: %v", err))
	}

	roles, err := extractRoles(*rolePath)
	if err != nil {
		panic(fmt.Sprintf("failed to extract roles: %v", err))
	}

	err = applyTemplatesAndRules(roles)
	if err != nil {
		panic(fmt.Sprintf("failed to apply templates and rules: %v", err))
	}

	var updatedYAML bytes.Buffer
	encoder := yaml.NewEncoder(&updatedYAML)
	encoder.SetIndent(2)

	for _, role := range roles {
		if err := encoder.Encode(role); err != nil {
			panic(fmt.Sprintf("failed to encode role: %v", err))
		}
	}
	err = encoder.Close()
	if err != nil {
		return
	}

	if err := os.WriteFile(*rolePath, updatedYAML.Bytes(), 0644); err != nil {
		panic(fmt.Sprintf("failed to write updated roles: %v", err))
	}
}

func extractRoles(inputFile string) ([]Role, error) {
	data, err := os.ReadFile(inputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %v", err)
	}

	var roles []Role
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	for {
		var role Role
		if err := decoder.Decode(&role); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed to decode YAML: %v", err)
		}
		roles = append(roles, role)
	}
	return roles, nil
}

func applyTemplatesAndRules(roles []Role) error {
	templateNames := map[string]string{
		"ClusterRole":        "hazelcast-platform-operator.hazelcastNodeReadRules",
		"operator-namespace": "hazelcast-platform-operator.operatorNamespaceRules",
		"watched":            "hazelcast-platform-operator.watchedNamespaceRules",
	}

	for i, role := range roles {
		var templateName string
		if role.Kind == "ClusterRole" {
			templateName = templateNames["ClusterRole"]
		} else if role.Kind == "Role" {
			switch role.Metadata.Namespace {
			case "operator-namespace":
				templateName = templateNames["operator-namespace"]
			case "watched":
				templateName = templateNames["watched"]
			}
		}

		if templateName != "" {
			ruleStr, err := executeTemplate(templateName)
			if err != nil {
				return err
			}
			roles[i].Rules = parseRules(ruleStr)
		}
	}
	return nil
}

func executeTemplate(name string) (string, error) {
	var buf bytes.Buffer
	err := tpl.ExecuteTemplate(&buf, name, nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func parseRules(rulesStr string) []Rule {
	var rules []Rule
	err := yaml.Unmarshal([]byte(rulesStr), &rules)
	if err != nil {
		panic(fmt.Sprintf("failed to unmarshal rules: %v", err))
	}
	return rules
}
