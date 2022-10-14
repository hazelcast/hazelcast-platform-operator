package main

import (
	"flag"
	"os"
	"time"

	"github.com/hazelcast/hazelcast-platform-operator/internal/mtls"
	"github.com/hazelcast/hazelcast-platform-operator/internal/phonehome"
	"github.com/hazelcast/hazelcast-platform-operator/internal/util"

	"github.com/hazelcast/hazelcast-platform-operator/controllers/hazelcast"
	"github.com/hazelcast/hazelcast-platform-operator/controllers/managementcenter"
	n "github.com/hazelcast/hazelcast-platform-operator/internal/naming"
	"github.com/hazelcast/hazelcast-platform-operator/internal/platform"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
		Development: util.IsDeveloperModeEnabled(),
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

	mtlsClient := mtls.NewClient(mgr.GetClient(), types.NamespacedName{
		Name: n.MTLSCertSecretName, Namespace: namespace,
	})
	if err := mgr.Add(mtlsClient); err != nil {
		setupLog.Error(err, "unable to create mtls client")
		os.Exit(1)
	}

	var metrics *phonehome.Metrics
	var phoneHomeTrigger chan struct{}
	if util.IsPhoneHomeEnabled() {
		phoneHomeTrigger = make(chan struct{}, 10)
		metrics = &phonehome.Metrics{
			UID:            util.GetOperatorID(cfg),
			CreatedAt:      time.Now(),
			PardotID:       util.GetPardotID(),
			Version:        util.GetOperatorVersion(),
			K8sDistibution: platform.GetDistribution(),
			K8sVersion:     platform.GetVersion(),
			Trigger:        phoneHomeTrigger,
		}
	}

	if err = hazelcast.NewHazelcastReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("Hazelcast"),
		mgr.GetScheme(),
		phoneHomeTrigger,
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Hazelcast")
		os.Exit(1)
	}

	if err = managementcenter.NewManagementCenterReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("Management Center"),
		mgr.GetScheme(),
		phoneHomeTrigger,
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ManagementCenter")
		os.Exit(1)
	}

	if err = hazelcast.NewHotBackupReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("HotBackup"),
		phoneHomeTrigger,
		mtlsClient,
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HotBackup")
		os.Exit(1)
	}

	if err = hazelcast.NewMapReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("Map"),
		mgr.GetScheme(),
		phoneHomeTrigger,
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Map")
		os.Exit(1)
	}

	if err = hazelcast.NewWanReplicationReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("WanReplication"),
		mgr.GetScheme(),
		phoneHomeTrigger,
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controllers", "WanReplication")
		os.Exit(1)
	}

	if err = hazelcast.NewCronHotBackupReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("CronHotBackup"),
		mgr.GetScheme(),
		phoneHomeTrigger,
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controllers", "CronHotBackup")
		os.Exit(1)
	}

	if err = hazelcast.NewMultiMapReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("MultiMap"),
		mgr.GetScheme(),
		phoneHomeTrigger,
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "MultiMap")
	}

	if err = hazelcast.NewTopicReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("Topic"),
		mgr.GetScheme(),
		phoneHomeTrigger,
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Topic")
		os.Exit(1)
	}

	if err = hazelcast.NewReplicatedMapReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("ReplicatedMap"),
		mgr.GetScheme(),
		phoneHomeTrigger,
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ReplicatedMap")
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
		phonehome.Start(mgr.GetClient(), metrics)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
