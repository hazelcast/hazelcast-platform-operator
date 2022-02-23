package test

import (
	"bufio"
	"context"
	"io"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
		sl, slErr := regexp.Compile("(It|Entry)\\((?:[^L]+|L(?:$|[^a]|a(?:$|[^b]|b(?:$|[^e]|e(?:$|[^l]|l(?:$|[^(]|\\((?:$|[^\"]|\"(?:$|[^s]|s(?:$|[^l]|l(?:$|[^o]|o(?:$|[^w]|w(?:$|[^\"]))))))))))))*$")
		fs, fsErr := regexp.Compile("(It|Entry)\\((?:[^L]+|L(?:$|[^a]|a(?:$|[^b]|b(?:$|[^e]|e(?:$|[^l]|l(?:$|[^(]|\\((?:$|[^\"]|\"(?:$|[^f]|f(?:$|[^a]|a(?:$|[^s]|s(?:$|[^t]|t(?:$|[^\"]))))))))))))*$")
		if slErr != nil || fsErr != nil {
			log.Fatal(err)
		}
		for scanner.Scan() {
			if sl.MatchString(scanner.Text()) && fs.MatchString(scanner.Text()) {
				testList = append(testList, strings.Join(strings.Fields(strings.TrimSpace(scanner.Text())), " "))
				labelCounter++
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
