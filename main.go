package main

import (
	"flag"
	"os"
	"time"

	"github.com/hazelcast/hazelcast-platform-operator/controllers/phonehome"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/util"
	"k8s.io/apimachinery/pkg/types"

	"github.com/hazelcast/hazelcast-platform-operator/controllers/hazelcast"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/managementcenter"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/platform"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

const (
	WatchNamespaceEnv = "WATCH_NAMESPACE"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(hazelcastcomv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Get watch namespace from environment variable.
	namespace, found := os.LookupEnv(WatchNamespaceEnv)
	if !found || namespace == "" {
		setupLog.Info("No namespace specified in the WATCH_NAMESPACE env variable, watching all namespaces")
	} else if namespace == "*" {
		setupLog.Info("Watching all namespaces")
		namespace = ""
	} else {
		setupLog.Info("Watching namespace: " + namespace)
	}

	cfg := ctrl.GetConfigOrDie()
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "8d830316.hazelcast.com",
		Namespace:              namespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	err = platform.FindAndSetPlatform(cfg)
	if err != nil {
		setupLog.Error(err, "unable to get platform info")
		os.Exit(1)
	}

	var metrics *phonehome.Metrics
	if util.IsPhoneHomeEnabled() {
		metrics = &phonehome.Metrics{
			UID:              util.GetOperatorID(cfg),
			CreatedAt:        time.Now(),
			HazelcastMetrics: make(map[types.UID]*phonehome.HazelcastMetrics),
			MCMetrics:        make(map[types.UID]*phonehome.MCMetrics),
			PardotID:         util.GetPardotID(),
			Version:          util.GetOperatorVersion(),
			K8sDistibution:   platform.GetDistribution(),
			K8sVersion:       platform.GetVersion(),
		}
	}

	if err = hazelcast.NewHazelcastReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("Hazelcast"),
		mgr.GetScheme(),
		metrics,
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Hazelcast")
		os.Exit(1)
	}

	if err = managementcenter.NewManagementCenterReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("Management Center"),
		mgr.GetScheme(),
		metrics,
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ManagementCenter")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	if util.IsPhoneHomeEnabled() {
		phonehome.Start(metrics)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
