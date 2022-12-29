# -*- mode: Python -*-

# For more on Extensions, see: https://docs.tilt.dev/extensions.html
load('ext://restart_process', 'docker_build_with_restart')
include('ext://cancel')

image_name='hz-operator-dev'

# to allow connection to remote k8s clusters
if os.getenv("ALLOW_REMOTE", default= "false").lower() == "true":
  allow_k8s_contexts(k8s_context())

# to allow using ttl.sh as container image registry, if your k8s cluster does not have one.
if os.getenv("USE_TTL_REG", default= "false").lower() == "true":
  registry_name='hpo-%s' % local('uuidgen')
  default_registry('ttl.sh/%s' % registry_name.strip("\n").lower())

go_compile_cmd='make build-tilt'
debug_enabled="false"
#
if os.getenv("DEBUG_ENABLED", default= "false").lower() == "true":
   go_compile_cmd='make build-tilt-debug'
   debug_enabled="true"

local_resource(
  'go-compile',
  go_compile_cmd,
  deps=['./main.go','api/','controllers/','internal/',],
  ignore=['api/v1alpha1/zz_generated.deepcopy.go*'],
)

local_resource(
  'run e2e-test in current namespace',
  'make test-e2e NAMESPACE=$(kubectl config view --minify --output "jsonpath={..namespace}")',
  trigger_mode=TRIGGER_MODE_MANUAL,
  auto_init=False,
)

docker_build_with_restart(
  ref=image_name,
  context='.',
  entrypoint='$GOPATH/bin/dlv --listen=0.0.0.0:40000 --api-version=2 --headless=true exec /manager-debug',
  dockerfile='./Dockerfile.tilt.debug',
  only=[
    './bin/tilt/manager-debug',
  ],
  live_update=[
    sync('./bin/tilt/manager-debug', '/manager-debug'),
  ],
)

if debug_enabled == "true":
  k8s_resource(workload='hazelcast-platform-controller-manager', port_forwards=[40000])


# This does not apply the operator deployment, it is done by docker_build_with_restart commmand
k8s_yaml(local("""make deploy APPLY_MANIFESTS=false \
              REMOVE_SECURITY_CONTEXT=true IMG=%s \
              DEBUG_ENABLED=%s
              NAMESPACE=$(kubectl config view --minify --output \"jsonpath={..namespace}\")""" %(image_name,debug_enabled)))

load('ext://uibutton', 'cmd_button','text_input',"location")
cmd_button('Undeploy operator',
            argv=['sh','-c', 'cd %s && make undeploy' % os.getcwd()],
            resource='hazelcast-platform-controller-manager',
            location=location.RESOURCE,
            icon_name='delete',
            text='Undeploy operator RBAC, CRD and deployment',
)

cmd_button('Delete CRs and PVCs',
            argv=['sh', '-c', '(kubectl delete $(kubectl get wanreplication,hotbackup,map,multimap,topic,replicatedmap,hazelcast,managementcenter -o name) 2> /dev/null || echo "No CRs" ) && kubectl delete pvc -l "app.kubernetes.io/managed-by=hazelcast-platform-operator"'],
            resource='hazelcast-platform-controller-manager',
            location=location.RESOURCE,
            icon_name='delete',
            text='Delete CRs and PVCs',
)

cmd_button('Restart deployment',
            argv=['sh', '-c', 'kubectl delete --grace-period 0 $(kubectl get po -l "app.kubernetes.io/managed-by=tilt" -o name)'],
            resource='hazelcast-platform-controller-manager',
            location=location.RESOURCE,
            icon_name='360',
            text='Restart deployment',
)

