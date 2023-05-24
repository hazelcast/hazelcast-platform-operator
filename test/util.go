package test

import (
	"bufio"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func GetPodLogs(ctx context.Context, pod types.NamespacedName, podLogOptions *corev1.PodLogOptions) io.ReadCloser {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		panic(err)
	}
	// creates the clientset
	clientset := kubernetes.NewForConfigOrDie(config)
	p, err := clientset.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, v1.GetOptions{})
	if err != nil {
		panic(err)
	}
	if p.Status.Phase != corev1.PodFailed && p.Status.Phase != corev1.PodRunning {
		panic("Unable to get pod logs for the pod in Phase " + p.Status.Phase)
	}
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, podLogOptions)
	podLogs, err := req.Stream(context.Background())
	if err != nil {
		panic(err)
	}
	return podLogs
}

func SpecLabelsChecker() {
	labelCounter := 0
	var testList []string
	var testSuites []string
	err := filepath.Walk("../../test", func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			testSuites = append(testSuites, path)
			return nil
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, testSuite := range testSuites {
		file, err := os.Open(testSuite)
		if err != nil {
			log.Fatal(err)
		}
		scanner := bufio.NewScanner(file)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 10*1024*1024)

		lbl, lblErr := regexp.Compile(`(^(.*?)\s+It|^(.*?)\s+Entry\()(.*?)Label\((.*?)$`)
		noLbl, noLblErr := regexp.Compile(`((^(.*?)\s+It)(.*?)func(.*?){$)|((^(.*?)\s+Entry\()(.*?),$)`)
		if lblErr != nil || noLblErr != nil {
			log.Fatal(err)
		}
		slowRegexp := regexp.MustCompile(`\bslow\b`)
		fastRegexp := regexp.MustCompile(`\bfast\b`)
		labelRegexp := regexp.MustCompile(`\bLabel\b`)
		for scanner.Scan() {
			text := scanner.Text()
			if lbl.MatchString(text) || noLbl.MatchString(text) {
				if !(slowRegexp.MatchString(text) || fastRegexp.MatchString(text)) || !labelRegexp.MatchString(text) {
					testList = append(testList, strings.Join(strings.Fields(strings.TrimSpace(text)), " "))
					labelCounter++
				}
			}
		}
		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
		file.Close()
	}
	if labelCounter > 0 {
		log.Fatalf("There are %d tests doesn't have Labels or has incorrect one. Possible lables are 'slow' and 'fast'. Add label to test using 'Label(\"slow\")' or 'Label(\"fast\"). "+
			"\nExample: it('should create Hazelcast cluster\", Label(\"slow\"), func()':\n \n * %s ", labelCounter, strings.Join(testList[:], "\n * "))
	}
}
