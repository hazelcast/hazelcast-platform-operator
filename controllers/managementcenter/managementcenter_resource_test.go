package managementcenter

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	hazelcastv1alpha1 "github.com/hazelcast/hazelcast-platform-operator/api/v1alpha1"
)

func Test_mcInitCmd(t *testing.T) {
	tests := []struct {
		name string
		mc   *hazelcastv1alpha1.ManagementCenter
		want string
	}{
		{
			name: "No Cluster Defined",
			mc: &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: hazelcastv1alpha1.ManagementCenterSpec{
					HazelcastClusters: []hazelcastv1alpha1.HazelcastClusterConfig{},
				},
			},
			want: "",
		},
		{
			name: "One Cluster Defined",
			mc: &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: hazelcastv1alpha1.ManagementCenterSpec{
					HazelcastClusters: []hazelcastv1alpha1.HazelcastClusterConfig{
						{
							Name:    "dev",
							Address: "hazelcast",
						},
					},
				},
			},
			want: "./bin/mc-conf.sh cluster add --lenient=true -H /data --client-config /config/dev.yaml",
		},
		{
			name: "Two Clusters Defined",
			mc: &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: hazelcastv1alpha1.ManagementCenterSpec{
					HazelcastClusters: []hazelcastv1alpha1.HazelcastClusterConfig{
						{
							Name:    "dev",
							Address: "hazelcast",
						},
						{
							Name:    "prod",
							Address: "hazelcast-prod",
						},
					},
				},
			},
			want: "./bin/mc-conf.sh cluster add --lenient=true -H /data --client-config /config/dev.yaml && ./bin/mc-conf.sh cluster add --lenient=true -H /data --client-config /config/prod.yaml",
		},
		{
			name: "No Cluster with LDAP",
			mc: &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: hazelcastv1alpha1.ManagementCenterSpec{
					HazelcastClusters: []hazelcastv1alpha1.HazelcastClusterConfig{},
					SecurityProviders: &hazelcastv1alpha1.SecurityProviders{
						LDAP: &hazelcastv1alpha1.LDAPProvider{
							URL:                   "ldap://10.124.0.27:1389",
							CredentialsSecretName: "ldap-credentials",
							GroupDN:               "ou=users,dc=example,dc=org",
							GroupSearchFilter:     "member={0}",
							NestedGroupSearch:     false,
							UserDN:                "ou=users,dc=example,dc=org",
							UserGroups:            []string{"readers"},
							MetricsOnlyGroups:     []string{"readers"},
							AdminGroups:           []string{"readers"},
							ReadonlyUserGroups:    []string{"readers"},
							UserSearchFilter:      "cn={0}",
						},
					},
				},
			},
			want: "./bin/hz-mc conf security reset -H /data && ./bin/hz-mc conf ldap configure -H /data --url=ldap://10.124.0.27:1389 --ldap-username=my-username --ldap-password=very-secret-password --user-dn=ou=users,dc=example,dc=org --group-dn=ou=users,dc=example,dc=org --user-search-filter=cn={0} --group-search-filter=member={0} --admin-groups=readers --read-write-groups=readers --read-only-groups=readers --metrics-only-groups=readers",
		},
		{
			name: "With Cluster and LDAP",
			mc: &hazelcastv1alpha1.ManagementCenter{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Spec: hazelcastv1alpha1.ManagementCenterSpec{
					HazelcastClusters: []hazelcastv1alpha1.HazelcastClusterConfig{
						{
							Name:    "dev",
							Address: "hazelcast",
						},
					},
					SecurityProviders: &hazelcastv1alpha1.SecurityProviders{
						LDAP: &hazelcastv1alpha1.LDAPProvider{
							URL:                   "ldap://10.124.0.27:1389",
							CredentialsSecretName: "ldap-credentials",
							GroupDN:               "ou=users,dc=example,dc=org",
							GroupSearchFilter:     "member={0}",
							NestedGroupSearch:     false,
							UserDN:                "ou=users,dc=example,dc=org",
							UserGroups:            []string{"readers"},
							MetricsOnlyGroups:     []string{"readers"},
							AdminGroups:           []string{"readers"},
							ReadonlyUserGroups:    []string{"readers"},
							UserSearchFilter:      "cn={0}",
						},
					},
				},
			},
			want: "./bin/mc-conf.sh cluster add --lenient=true -H /data --client-config /config/dev.yaml && ./bin/hz-mc conf security reset -H /data && ./bin/hz-mc conf ldap configure -H /data --url=ldap://10.124.0.27:1389 --ldap-username=my-username --ldap-password=very-secret-password --user-dn=ou=users,dc=example,dc=org --group-dn=ou=users,dc=example,dc=org --user-search-filter=cn={0} --group-search-filter=member={0} --admin-groups=readers --read-write-groups=readers --read-only-groups=readers --metrics-only-groups=readers",
		},
	}
	for _, tt := range tests {
		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ldap-credentials",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"username": []byte("my-username"),
				"password": []byte("very-secret-password"),
			},
		}
		c := fakeK8sClient(secret)
		t.Run(tt.name, func(t *testing.T) {
			if got := buildMcInitCmd(context.TODO(), tt.mc, c, logr.Discard()); got != tt.want {
				t.Errorf("clusterAddCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func fakeK8sClient(initObjs ...client.Object) client.Client {
	coreScheme, _ := (&scheme.Builder{GroupVersion: corev1.SchemeGroupVersion}).
		Register(&v1.ClusterRole{}, &v1.ClusterRoleBinding{}, &corev1.Secret{}).
		Build()
	return fake.NewClientBuilder().WithScheme(coreScheme).WithObjects(initObjs...).Build()
}
