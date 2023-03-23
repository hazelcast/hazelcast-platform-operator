#!/bin/bash

numberOfAllResources() {
  # number of all resources excepting `kubernetes` svc
  kubectl get all --namespace="$1" -o json | \
    jq '.items | map(select((.kind | contains("Service")) and (.metadata.name | contains("kubernetes")) | not)) | length'
}

numberOfPvc() {
  # number of PVCs
  kubectl get pvc --namespace="$1" -o json | \
    jq '.items | length'
}

listOfBoundedPv(){
  # space separated list of PVs which are bounded to PVCs in given namespace
  kubectl get pvc --namespace="$1" -o json | \
    jq -r '.items[].spec.volumeName' | \
    tr '\n' ' '
}

ns="$1"

if [ -z "$ns" ];
then
    echo "Namespace is not passed"
    exit 1
fi

yellow='\033[1;33m'
cyan='\033[0;36m'
green='\033[1;32m'

echo "${yellow}namespace: '$ns'"

while [ "$(numberOfAllResources "$ns")" -gt 0 ]
do
  echo "${cyan}kubectl delete all${green}"
  kubectl delete all --all --namespace="$ns"
  sleep 3
done

if [ "$(numberOfPvc "$ns")" -gt 0 ];
then
  pvList="$(listOfBoundedPv "$ns")"
  echo "${cyan}kubectl delete PVCs${green}"
  kubectl delete pvc --all --namespace="$ns"
  echo "${cyan}kubectl delete PV $pvList${green}"
  kubectl delete pv $pvList || true
fi

echo '\033[0m'
