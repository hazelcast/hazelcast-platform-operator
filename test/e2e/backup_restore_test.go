package e2e

import (
	"context"
	"strconv"
	. "time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	hazelcastcomv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
	codecTypes "github.com/hazelcast/hazelcast-platform-operator/internal/protocol/types"
	"github.com/hazelcast/hazelcast-platform-operator/test"
	hazelcastconfig "github.com/hazelcast/hazelcast-platform-operator/test/e2e/config/hazelcast"
)

var _ = Describe("Hazelcast CR with Persistence feature enabled", Group("backup_restore"), func() {
	localPort := strconv.Itoa(8400 + GinkgoParallelProcess())
	backupRestore := func(hazelcast *hazelcastcomv1alpha1.Hazelcast, hotBackup *hazelcastcomv1alpha1.HotBackup, useBucketConfig bool) {
		By("creating cluster with backup enabled")
		CreateHazelcastCR(hazelcast)
		evaluateReadyMembers(hzLookupKey)

		By("creating the map config and adding entries")
		m := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)
		Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
		assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
		fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 10)

		By("triggering backup")
		t := Now()
		Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
		hotBackup = assertHotBackupSuccess(hotBackup, 1*Minute)

		By("checking if backup status is correct")
		assertCorrectBackupStatus(hotBackup, GetBackupSequence(t, hzLookupKey))

		By("adding new entries after backup")
		fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 15)

		By("removing Hazelcast CR")
		RemoveHazelcastCR(hazelcast)

		By("creating cluster from backup")
		restoredHz := hazelcastconfig.HazelcastRestore(hazelcast, restoreConfig(hotBackup, useBucketConfig))
		CreateHazelcastCR(restoredHz)
		evaluateReadyMembers(hzLookupKey)

		By("checking the cluster state and map size")
		assertHazelcastRestoreStatus(restoredHz, hazelcastcomv1alpha1.RestoreSucceeded)
		assertClusterStatePortForward(context.Background(), restoredHz, localPort, codecTypes.ClusterStateActive)
		waitForMapSizePortForward(context.Background(), restoredHz, localPort, m.MapName(), 10, 1*Minute)
	}
	AfterEach(func() {
		GinkgoWriter.Printf("Aftereach start time is %v\n", Now().String())
		if skipCleanup() {
			return
		}
		DeleteAllOf(&hazelcastcomv1alpha1.Map{}, &hazelcastcomv1alpha1.MapList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Cache{}, &hazelcastcomv1alpha1.CacheList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.Hazelcast{}, nil, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.CronHotBackup{}, &hazelcastcomv1alpha1.CronHotBackupList{}, hzNamespace, labels)
		DeleteAllOf(&hazelcastcomv1alpha1.HotBackup{}, &hazelcastcomv1alpha1.HotBackupList{}, hzNamespace, labels)
		deletePVCs(hzLookupKey)
		assertDoesNotExist(hzLookupKey, &hazelcastcomv1alpha1.Hazelcast{})
		GinkgoWriter.Printf("Aftereach end time is %v\n", Now().String())
	})

	Context("The hot backup process", func() {
		It("triggers successfully", Tag(Fast|EE|AnyCloud), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}

			setLabelAndCRName("br-1")
			clusterSize := int32(3)

			hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
			hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastcomv1alpha1.MostRecent
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			By("creating HotBackup CR")
			t := Now()
			hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
			Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

			By("checking the HotBackup creation sequence")
			logs := InitLogs(t, hzLookupKey)
			logReader := test.NewLogReader(logs)
			defer logReader.Close()
			test.EventuallyInLogs(logReader, 15*Second, logInterval).
				Should(ContainSubstring("ClusterStateChange{type=class com.hazelcast.cluster.ClusterState, newState=PASSIVE}"))
			test.EventuallyInLogsUnordered(logReader, 15*Second, logInterval).
				Should(ContainElements(
					ContainSubstring("Starting new hot backup with sequence"),
					ContainSubstring("ClusterStateChange{type=class com.hazelcast.cluster.ClusterState, newState=ACTIVE}"),
					MatchRegexp(`(.*) Backup of hot restart store (.*?) finished in [0-9]* ms`)))

			assertHotBackupSuccess(hotBackup, 1*Minute)
		})

		It("starts after the cluster becomes ready", Tag(Fast|EE|AnyCloud), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			setLabelAndCRName("br-2")
			clusterSize := int32(1)
			hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
			hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = hazelcastcomv1alpha1.MostRecent

			By("creating HotBackup CR")
			hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
			Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

			assertHotBackupStatus(hotBackup, hazelcastcomv1alpha1.HotBackupPending, 1*Minute)

			By("creating cluster with backup enabled")
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			assertHotBackupSuccess(hotBackup, 1*Minute)
		})

		It("is interrupted when the HotBackup CR is deleted", Tag(Fast|EE|AnyCloud), func() {
			setLabelAndCRName("br-3")
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			ctx := context.Background()
			bucketURI := "gs://operator-e2e-external-backup"
			secretName := "br-secret-gcp"
			mapSizeInMb := 1024
			pvcSizeInMb := mapSizeInMb * 2 // Taking backup duplicates the used storage
			clusterSize := int32(3)

			By("creating cluster with external backup enabled")
			hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
			hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
			}
			hazelcast.Spec.Resources = &corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")},
			}
			hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")}[0]
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			By("creating the map config")
			m := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)
			Expect(k8sClient.Create(ctx, m)).Should(Succeed())
			assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)

			By("filling the Map")
			FillMapBySizeInMb(ctx, m.MapName(), mapSizeInMb, mapSizeInMb, hazelcast)

			t := Now()

			By("creating HotBackup CR")
			hotBackup := hazelcastconfig.HotBackupBucket(hbLookupKey, hazelcast.Name, labels, bucketURI, secretName)
			Expect(k8sClient.Create(ctx, hotBackup)).Should(Succeed())

			By("checking hazelcast logs if backup started")
			hzLogs := InitLogs(t, hzLookupKey)
			hzLogReader := test.NewLogReader(hzLogs)
			defer hzLogReader.Close()
			test.EventuallyInLogs(hzLogReader, 10*Second, logInterval).Should(ContainSubstring("Starting new hot backup with sequence"))
			test.EventuallyInLogs(hzLogReader, 10*Second, logInterval).Should(MatchRegexp(`Backup of hot restart store (.*?) finished in [0-9]* ms`))

			By("checking agent logs if upload is started")
			agentLogs := SidecarAgentLogs(t, hzLookupKey)
			agentLogReader := test.NewLogReader(agentLogs)
			defer agentLogReader.Close()
			test.EventuallyInLogs(agentLogReader, 10*Second, logInterval).Should(ContainSubstring("Starting new task"))
			test.EventuallyInLogs(agentLogReader, 10*Second, logInterval).Should(ContainSubstring("task is started"))
			test.EventuallyInLogs(agentLogReader, 10*Second, logInterval).Should(ContainSubstring("task successfully read secret"))
			test.EventuallyInLogs(agentLogReader, 10*Second, logInterval).Should(ContainSubstring("task is in progress"))

			By("get hotbackup object")
			hb := &hazelcastcomv1alpha1.HotBackup{}
			err := k8sClient.Get(context.Background(), types.NamespacedName{Name: hotBackup.Name, Namespace: hzNamespace}, hb)
			Expect(err).ToNot(HaveOccurred())
			Expect(hb.Status.State).Should(Equal(hazelcastcomv1alpha1.HotBackupInProgress))

			By("delete hotbackup to cancel backup process")
			err = k8sClient.Delete(ctx, hb)
			Expect(err).ToNot(HaveOccurred())

			By("checking agent logs if upload canceled")
			test.EventuallyInLogs(agentLogReader, 10*Second, logInterval).Should(ContainSubstring("canceling task"))
		})

		It("fails when external backup credentials are incorrect", Tag(Fast|EE|AnyCloud), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			setLabelAndCRName("br-4")
			ctx := context.Background()
			clusterSize := int32(1)
			var pvcSizeInMb = 1
			var bucketURI = "gs://operator-e2e-external-backup"
			var secretName = "br-incorrect-secret-gcp"
			var credential = "{" +
				"  \"type\": \"service_account\"," +
				"  \"project_id\": \"project\"," +
				"  \"private_key_id\": \"12345678910111213\"," +
				"  \"private_key\": \"-----BEGIN PRIVATE KEY-----\"," +
				"  \"client_email\": \"sa@project.iam.gserviceaccount.com\"," +
				"  \"client_id\": \"123456789\"," +
				"  \"auth_uri\": \"https://accounts.google.com/o/oauth2/auth\"," +
				"  \"token_uri\": \"https://oauth2.googleapis.com/token\"," +
				"  \"auth_provider_x509_cert_url\": \"https://www.googleapis.com/oauth2/v1/certs\"," +
				"  \"client_x509_cert_url\": \"https://www.googleapis.com/robot/v1/metadata/x509/sa%40project.iam.gserviceaccount.com\"" +
				"}"

			By("creating cluster with external backup enabled")
			hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
			hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")}[0]

			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			By("create bucket credential secret")
			secret := corev1.Secret{}
			secret.StringData = map[string]string{
				"google-credentials-path": credential,
			}
			secret.Name = secretName
			secret.Namespace = hazelcast.Namespace
			Eventually(func() error {
				err := k8sClient.Create(ctx, &secret)
				if errors.IsAlreadyExists(err) {
					return nil
				}
				return err
			}, Minute, interval).Should(Succeed())
			assertExists(types.NamespacedName{Namespace: secret.Namespace, Name: secret.Name}, &secret)

			By("triggering the backup")
			hotBackup := hazelcastconfig.HotBackupBucket(hbLookupKey, hazelcast.Name, labels, bucketURI, secretName)
			Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())

			Eventually(func() hazelcastcomv1alpha1.HotBackupState {
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: hotBackup.Namespace, Name: hotBackup.Name}, hotBackup)
				Expect(err).ToNot(HaveOccurred())
				return hotBackup.Status.State
			}, 20*Second, interval).Should(Equal(hazelcastcomv1alpha1.HotBackupFailure))
			Expect(hotBackup.Status.Message).Should(ContainSubstring("Upload failed"))
		})

		It("triggers multiple times using CronHotBackup", Tag(Fast|EE|AnyCloud), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			setLabelAndCRName("br-5")
			By("creating cron hot backup")
			hbSpec := &hazelcastcomv1alpha1.HotBackupSpec{}
			chb := hazelcastconfig.CronHotBackup(hzLookupKey, "*/5 * * * * *", hbSpec, labels)
			Expect(k8sClient.Create(context.Background(), chb)).Should(Succeed())

			By("waiting cron hot backup two create two backups")
			Eventually(func() int {
				hbl := &hazelcastcomv1alpha1.HotBackupList{}
				err := k8sClient.List(context.Background(), hbl, client.InNamespace(chb.Namespace), client.MatchingLabels(labels))
				if err != nil {
					return 0
				}
				return len(hbl.Items)
			}, 11*Second, 1*Second).Should(Equal(2))

			By("deleting the cron hot backup")
			DeleteAllOf(&hazelcastcomv1alpha1.CronHotBackup{}, &hazelcastcomv1alpha1.CronHotBackupList{}, hzNamespace, labels)

			By("seeing hot backup CRs are also deleted")
			hbl := &hazelcastcomv1alpha1.HotBackupList{}
			Expect(k8sClient.List(context.Background(), hbl, client.InNamespace(chb.Namespace), client.MatchingLabels(labels))).Should(Succeed())
			Expect(len(hbl.Items)).To(Equal(0))
		})

		It("should backup, restore and backup data again successfully", Tag(Slow|EE|AnyCloud), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			setLabelAndCRName("br-6")
			var mapSizeInMb = 1072
			var additionalEntries = 111
			var pvcSizeInMb = mapSizeInMb * 2 // Taking backup duplicates the used storage
			var expectedMapSize = int(float64(mapSizeInMb) * 128)
			ctx := context.Background()
			clusterSize := int32(3)

			By("creating Hazelcast cluster")
			hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
			hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
			}
			hazelcast.Spec.Resources = &corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")},
			}
			hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")}[0]
			CreateHazelcastCR(hazelcast)

			By("creating the map config and putting entries")
			dm := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)
			Expect(k8sClient.Create(context.Background(), dm)).Should(Succeed())
			assertMapStatus(dm, hazelcastcomv1alpha1.MapSuccess)
			FillMapBySizeInMb(ctx, dm.MapName(), mapSizeInMb, mapSizeInMb, hazelcast)

			By("creating first HotBackup CR")
			hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
			Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
			assertHotBackupSuccess(hotBackup, 10*Minute)

			By("deleting Hazelcast cluster")
			RemoveHazelcastCR(hazelcast)

			By("creating new Hazelcast cluster from the first backup")
			hazelcast = hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
			hazelcast.Spec.Persistence.Restore = hazelcastcomv1alpha1.RestoreConfiguration{
				HotBackupResourceName: hotBackup.Name,
			}
			hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
			}
			hazelcast.Spec.Resources = &corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")},
			}
			hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")}[0]
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			By("putting entries after first restore")
			err := FillMapByEntryCount(ctx, hzLookupKey, false, dm.MapName(), additionalEntries)
			Expect(err).To(BeNil())

			By("creating second HotBackup CR")
			hotBackup2 := hazelcastconfig.HotBackup(hbLookupKey2, hazelcast.Name, labels)
			Expect(k8sClient.Create(context.Background(), hotBackup2)).Should(Succeed())
			assertHotBackupSuccess(hotBackup2, 10*Minute)

			By("checking the cluster state and map size")
			assertHazelcastRestoreStatus(hazelcast, hazelcastcomv1alpha1.RestoreSucceeded)
			assertClusterStatePortForward(context.Background(), hazelcast, localPort, codecTypes.ClusterStateActive)
			WaitForMapSize(context.Background(), hzLookupKey, dm.MapName(), expectedMapSize+additionalEntries, 10*Minute)
		})

	})

	Context("Restoring and verifying data", func() {
		It("should restore from LocalBackup using PVC and HotBackupResourceName", Tag(Fast|EE|AnyCloud), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			setLabelAndCRName("br-7")
			clusterSize := int32(3)

			hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
			hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
			backupRestore(hazelcast, hotBackup, false)
		})

		It("should restore 3 GB from an external backup using a GCP bucket", Tag(Slow|EE|AnyCloud), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			setLabelAndCRName("br-8")
			ctx := context.Background()
			var mapSizeInMb = 3072
			var pvcSizeInMb = mapSizeInMb * 2 // Taking backup duplicates the used storage
			var bucketURI = "gs://operator-e2e-external-backup"
			var secretName = "br-secret-gcp"
			expectedMapSize := int(float64(mapSizeInMb) * 128)
			clusterSize := int32(3)

			By("creating cluster with external backup enabled")
			hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
			hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
			}
			hazelcast.Spec.Resources = &corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")},
			}
			hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")}[0]

			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			By("creating the map config")
			dm := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)
			Expect(k8sClient.Create(context.Background(), dm)).Should(Succeed())
			assertMapStatus(dm, hazelcastcomv1alpha1.MapSuccess)

			By("filling the Map")
			FillMapBySizeInMb(ctx, dm.MapName(), mapSizeInMb, mapSizeInMb, hazelcast)
			By("triggering the backup")
			hotBackup := hazelcastconfig.HotBackupBucket(hbLookupKey, hazelcast.Name, labels, bucketURI, secretName)
			Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
			assertHotBackupSuccess(hotBackup, 20*Minute)

			By("putting entries after backup")
			err := FillMapByEntryCount(ctx, hzLookupKey, false, dm.MapName(), 111)
			Expect(err).To(BeNil())
			By("deleting Hazelcast cluster")
			RemoveHazelcastCR(hazelcast)
			deletePVCs(hzLookupKey)

			By("creating cluster from external backup")
			hazelcast = hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
			hazelcast.Spec.Persistence.Restore = hazelcastcomv1alpha1.RestoreConfiguration{
				HotBackupResourceName: hotBackup.Name}
			hazelcast.Spec.ExposeExternally = &hazelcastcomv1alpha1.ExposeExternallyConfiguration{
				Type:                 hazelcastcomv1alpha1.ExposeExternallyTypeSmart,
				DiscoveryServiceType: corev1.ServiceTypeLoadBalancer,
				MemberAccess:         hazelcastcomv1alpha1.MemberAccessLoadBalancer,
			}
			hazelcast.Spec.Resources = &corev1.ResourceRequirements{
				Limits: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceMemory: resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")},
			}
			hazelcast.Spec.Persistence.Pvc.RequestStorage = &[]resource.Quantity{resource.MustParse(strconv.Itoa(pvcSizeInMb) + "Mi")}[0]
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			By("checking the cluster state and map size")
			assertHazelcastRestoreStatus(hazelcast, hazelcastcomv1alpha1.RestoreSucceeded)
			assertClusterStatePortForward(context.Background(), hazelcast, localPort, codecTypes.ClusterStateActive)
			WaitForMapSize(context.Background(), hzLookupKey, dm.MapName(), expectedMapSize, 30*Minute)
		})

		It("should restore multiple times from HotBackupResourceName", Tag(Slow|EE|AnyCloud), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			setLabelAndCRName("br-8")
			clusterSize := int32(3)

			By("creating cluster with external backup enabled")
			hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)

			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			By("creating the map config")
			m := hazelcastconfig.PersistedMap(mapLookupKey, hazelcast.Name, labels)
			Expect(k8sClient.Create(context.Background(), m)).Should(Succeed())
			assertMapStatus(m, hazelcastcomv1alpha1.MapSuccess)
			fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 10)

			By("triggering first backup as local")
			hotBackup := hazelcastconfig.HotBackupBucket(hbLookupKey, hazelcast.Name, labels, "", "")
			Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
			hotBackup = assertHotBackupSuccess(hotBackup, 1*Minute)
			fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 10)

			By("triggering second backup as external")
			hbLookupKey2 := types.NamespacedName{Name: hbLookupKey.Name + "2", Namespace: hbLookupKey.Namespace}
			hotBackup2 := hazelcastconfig.HotBackupBucket(hbLookupKey2, hazelcast.Name, labels, "gs://operator-e2e-external-backup", "br-secret-gcp")
			Expect(k8sClient.Create(context.Background(), hotBackup2)).Should(Succeed())
			hotBackup2 = assertHotBackupSuccess(hotBackup2, 1*Minute)
			fillTheMapDataPortForward(context.Background(), hazelcast, localPort, m.MapName(), 10)

			RemoveHazelcastCR(hazelcast)

			By("creating cluster from from first backup")
			hazelcast = hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
			hazelcast.Spec.Persistence.Restore = hazelcastcomv1alpha1.RestoreConfiguration{
				HotBackupResourceName: hotBackup.Name,
			}
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			By("checking the cluster state and map size")
			assertHazelcastRestoreStatus(hazelcast, hazelcastcomv1alpha1.RestoreSucceeded)
			assertClusterStatePortForward(context.Background(), hazelcast, localPort, codecTypes.ClusterStateActive)
			waitForMapSizePortForward(context.Background(), hazelcast, localPort, m.MapName(), 10, 1*Minute)

			RemoveHazelcastCR(hazelcast)

			By("creating cluster from from second backup")
			hazelcast = hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
			hazelcast.Spec.Persistence.Restore = hazelcastcomv1alpha1.RestoreConfiguration{
				HotBackupResourceName: hotBackup2.Name,
			}
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			By("checking the cluster state and map size")
			assertHazelcastRestoreStatus(hazelcast, hazelcastcomv1alpha1.RestoreSucceeded)
			assertClusterStatePortForward(context.Background(), hazelcast, localPort, codecTypes.ClusterStateActive)
			waitForMapSizePortForward(context.Background(), hazelcast, localPort, m.MapName(), 20, 1*Minute)
		})

		It("should check cache entry persistence after HotBackup", Tag(Slow|EE|AnyCloud), func() {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			setLabelAndCRName("br-9")
			clusterSize := int32(3)

			hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
			CreateHazelcastCR(hazelcast)
			evaluateReadyMembers(hzLookupKey)

			By("creating the cache config")
			cache := hazelcastconfig.DefaultCache(chLookupKey, hazelcast.Name, labels)
			cache.Spec.PersistenceEnabled = true
			Expect(k8sClient.Create(context.Background(), cache)).Should(Succeed())
			assertDataStructureStatus(chLookupKey, hazelcastcomv1alpha1.DataStructureSuccess, cache)

			By("filling the cache with entries")
			entryCount := 10
			fillCachePortForward(hazelcast, cache.GetDSName(), localPort, entryCount)
			validateCacheEntriesPortForward(hazelcast, localPort, cache.GetDSName(), entryCount)

			By("creating HotBackup CR")
			hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
			Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
			assertHotBackupSuccess(hotBackup, 1*Minute)

			By("filling the cache with entries after backup")
			fillCachePortForward(hazelcast, cache.GetDSName(), localPort, entryCount)

			RemoveHazelcastCR(hazelcast)

			By("creating new Hazelcast cluster from existing backup")
			hazelcast = hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
			hazelcast.Spec.Persistence.Restore = hazelcastcomv1alpha1.RestoreConfiguration{
				HotBackupResourceName: hotBackup.Name,
			}

			Expect(k8sClient.Create(context.Background(), hazelcast)).Should(Succeed())
			evaluateReadyMembers(hzLookupKey)
			assertHazelcastRestoreStatus(hazelcast, hazelcastcomv1alpha1.RestoreSucceeded)

			By("checking the cache entries")
			validateCacheEntriesPortForward(hazelcast, localPort, cache.GetDSName(), entryCount)
		})

		DescribeTable("when restoring from ExternalBackup", func(bucketURI, secretName string, useBucketConfig bool) {
			if !ee {
				Skip("This test will only run in EE configuration")
			}
			setLabelAndCRName("br-10")
			By("creating cluster with backup enabled")
			clusterSize := int32(3)

			hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
			hotBackup := hazelcastconfig.HotBackupBucket(hbLookupKey, hazelcast.Name, labels, bucketURI, secretName)
			backupRestore(hazelcast, hotBackup, useBucketConfig)
		},
			Entry("using AWS S3 bucket HotBackupResourceName", Tag(Slow|EE|AnyCloud), "s3://operator-e2e-external-backup", "br-secret-s3", false),
			Entry("using GCP bucket HotBackupResourceName", Tag(Slow|EE|AnyCloud), "gs://operator-e2e-external-backup", "br-secret-gcp", false),
			Entry("using Azure bucket HotBackupResourceName", Tag(Slow|EE|AnyCloud), "azblob://operator-e2e-external-backup", "br-secret-az", false),
			Entry("using GCP bucket restore from BucketConfig", Tag(Slow|EE|AnyCloud), "gs://operator-e2e-external-backup", "br-secret-gcp", true),
		)
	})

	Context("Startup actions configuration", func() {
		DescribeTable("should start the cluster successfully triggering",
			func(action hazelcastcomv1alpha1.PersistenceStartupAction, dataPolicy hazelcastcomv1alpha1.DataRecoveryPolicyType) {
				if !ee {
					Skip("This test will only run in EE configuration")
				}
				setLabelAndCRName("br-11")
				clusterSize := int32(3)

				hazelcast := hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
				CreateHazelcastCR(hazelcast)
				evaluateReadyMembers(hzLookupKey)

				By("creating HotBackup CR")
				t := Now()
				hotBackup := hazelcastconfig.HotBackup(hbLookupKey, hazelcast.Name, labels)
				Expect(k8sClient.Create(context.Background(), hotBackup)).Should(Succeed())
				assertHotBackupSuccess(hotBackup, 1*Minute)

				seq := GetBackupSequence(t, hzLookupKey)
				RemoveHazelcastCR(hazelcast)

				By("creating new Hazelcast cluster from existing backup with 2 members")
				baseDir := "/data/hot-restart/hot-backup/backup-" + seq

				hazelcast = hazelcastconfig.HazelcastPersistencePVC(hzLookupKey, clusterSize, labels)
				hazelcast.Spec.Persistence.BaseDir = baseDir
				hazelcast.Spec.ClusterSize = &[]int32{2}[0]
				hazelcast.Spec.Persistence.DataRecoveryTimeout = 60
				hazelcast.Spec.Persistence.ClusterDataRecoveryPolicy = dataPolicy
				hazelcast.Spec.Persistence.StartupAction = action
				CreateHazelcastCR(hazelcast)
				evaluateReadyMembers(hzLookupKey)
				assertClusterStatePortForward(context.Background(), hazelcast, localPort, codecTypes.ClusterStateActive)
			},
			Entry("ForceStart action and FullRecovery policy", Tag(Slow|EE|AnyCloud), hazelcastcomv1alpha1.ForceStart, hazelcastcomv1alpha1.FullRecovery),
			Entry("PartialStart action and MostRecent policy", Tag(Slow|EE|AnyCloud), hazelcastcomv1alpha1.PartialStart, hazelcastcomv1alpha1.MostRecent),
		)
	})

})
