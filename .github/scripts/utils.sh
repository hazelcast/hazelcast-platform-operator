#!/bin/bash
get_image()
{
    local PUBLISHED=$1
    local PROJECT_ID=$2
    local VERSION=$3
    local RHEL_API_KEY=$4

    if [[ $PUBLISHED == "published" ]]; then
        local PUBLISHED_FILTER="repositories.published==true"
    elif [[ $PUBLISHED == "not_published" ]]; then
        local PUBLISHED_FILTER="repositories.published!=true"
    else
        echo "Need first parameter as 'published' or 'not_published'." ; return 1
    fi

    local FILTER="filter=deleted==false;${PUBLISHED_FILTER};repositories.tags.name==${VERSION}"
    local INCLUDE="include=total,data.repositories.tags.name,data.scan_status,data._id"

    local RESPONSE=$( \
        curl --silent \
             --request GET \
             --header "X-API-KEY: ${RHEL_API_KEY}" \
             "https://catalog.redhat.com/api/containers/v1/projects/certification/id/${PROJECT_ID}/images?${FILTER}&${INCLUDE}")

    echo "${RESPONSE}"
}

wait_for_container_scan()
{
    local PROJECT_ID=$1
    local VERSION=$2
    local RHEL_API_KEY=$3
    local TIMEOUT_IN_MINS=$4

    local IS_PUBLISHED=$(get_image published "${PROJECT_ID}" "${VERSION}" "${RHEL_API_KEY}" | jq -r '.total')
    if [[ $IS_PUBLISHED == "1" ]]; then
        echo "Image is already published, exiting"
        return 0
    fi

    local NOF_RETRIES=$(( $TIMEOUT_IN_MINS / 2 ))
    # Wait until the image is scanned
    for i in `seq 1 ${NOF_RETRIES}`; do
        local IMAGE=$(get_image not_published "${PROJECT_ID}" "${VERSION}" "${RHEL_API_KEY}")
        local SCAN_STATUS=$(echo "$IMAGE" | jq -r '.data[0].scan_status')

        if [[ $SCAN_STATUS == "in progress" ]]; then
            echo "Scanning in progress, waiting..."
        elif [[ $SCAN_STATUS == "null" ]];  then
            echo "Image is still not present in the registry!"
        elif [[ $SCAN_STATUS == "passed" ]]; then
            echo "Scan passed!" ; return 0
        else
            echo "Scan failed!" ; return 1
        fi

        if [[ $i == $NOF_RETRIES ]]; then
            echo "Timeout! Scan could not be finished"
            return 42
        fi
        sleep 120
    done
}

wait_for_container_publish()
{
    local PROJECT_ID=$1
    local VERSION=$2
    local RHEL_API_KEY=$3
    local TIMEOUT_IN_MINS=$4

    local NOF_RETRIES=$(( $TIMEOUT_IN_MINS * 6 ))
    # Wait until the image is published
    for i in `seq 1 ${NOF_RETRIES}`; do
        local IS_PUBLISHED=$(get_image published "${PROJECT_ID}" "${VERSION}" "${RHEL_API_KEY}" | jq -r '.total')

        if [[ $IS_PUBLISHED == "1" ]]; then
            echo "Image is published, exiting."
            return 0
        else
            echo "Image is still not published, waiting..."
        fi

        if [[ $i == $NOF_RETRIES ]]; then
            echo "Timeout! Publish could not be finished"
            return 42
        fi
        sleep 10
    done
}

checking_image_grade()
{
    local PROJECT_ID=$1
    local VERSION=$2
    local RHEL_API_KEY=$3
    local TIMEOUT_IN_MINS=$4
    FILTER="filter=deleted==false;repositories.published==false;repositories.tags.name==${VERSION}"
    INCLUDE="include=data.freshness_grades.grade&include=data.freshness_grades.end_date"

    local NOF_RETRIES=$(( $TIMEOUT_IN_MINS * 3 ))
    for i in `seq 1 ${NOF_RETRIES}`; do

    local GRADE_PRESENCE=$(curl -s -X 'GET' \
      "https://catalog.redhat.com/api/containers/v1/projects/certification/id/${PROJECT_ID}/images?${FILTER}&page_size=100&page=0" \
      -H "accept: application/json" \
      -H "X-API-KEY: ${RHEL_API_KEY}" | jq -r -e '.data[0].freshness_grades | length')

        if [[ ${GRADE_PRESENCE} -ne "0" ]]; then
            GRADE_A=$(curl -s -X 'GET' \
            "https://catalog.redhat.com/api/containers/v1/projects/certification/id/${PROJECT_ID}/images?${INCLUDE}&${FILTER}&page_size=100&page=0" \
            -H "accept: application/json" \
            -H "X-API-KEY: ${RHEL_API_KEY}" | jq -e '.data[0].freshness_grades[] | select(.grade =="A") | length')
        if [[ ${GRADE_A} -ne "0" ]]; then
            echo "The submitted image got a Health Index 'A'."
            return 0
        else
            echo "The submitted image didn’t get a Health Index 'A'."
            exit 1
        fi
        else
            echo "The submitted image still has unknown Health Index Image, waiting..."
        fi
        if [[ ${i} == ${NOF_RETRIES} ]]; then
            echo "Timeout! The submitted image has 'Unknown' Health Index."
            return 42
        fi
        sleep 20
    done
}

# This function will delete completed 'pages-build-deployment' runs using 'run_number' located in a custom head_commit message
cleanup_page_publish_runs()
{
    sleep 2
    local GITHUB_REPOSITORY=$1
    local JOB_NAME=$2
    local MASTER_JOB_RUN_NUMBER=$3
    local TIMEOUT_IN_MINS=$4
    local NOF_RETRIES=$(( $TIMEOUT_IN_MINS * 3 ))
    local WORKFLOW_ID=$(gh api repos/${GITHUB_REPOSITORY}/actions/workflows | jq '.workflows[] | select(.["name"] | contains("'${JOB_NAME}'")) | .id')

    for i in `seq 1 ${NOF_RETRIES}`; do
            RUN_ID=$(gh api repos/${GITHUB_REPOSITORY}/actions/workflows/${WORKFLOW_ID}/runs --paginate | jq '.workflow_runs[] | select(.["status"] | contains("completed")) | select(.head_commit.message | select(contains("'${MASTER_JOB_RUN_NUMBER}'"))) | .id')
            if [[ ${RUN_ID} -ne "" ]]; then
                    echo "Deleting Run ID $RUN_ID for the workflow ID $WORKFLOW_ID"
                    gh api repos/${GITHUB_REPOSITORY}/actions/runs/${RUN_ID} -X DELETE >/dev/null
                return 0
            else
                echo "The '${JOB_NAME}' job that was triggered by job with run number '${MASTER_JOB_RUN_NUMBER}' is not finished yet. Waiting..."
            fi
            if [[ ${i} == ${NOF_RETRIES} ]]; then
                echo "Timeout! 'pages-build-deployment' job still not completed."
                return 42
            fi
            sleep 20
    done
}

# The function waits until all EKS stacks will be deleted. Takes 2 arguments - cluster name and timeout.
wait_for_eks_stack_deleted()
{
    local CLUSTER_NAME=$1
    local TIMEOUT_IN_MINS=$2
    local NOF_RETRIES=$(( $TIMEOUT_IN_MINS * 3 ))
    LIST_OF_STACKS=$(aws cloudformation describe-stacks --no-paginate --query \
          'Stacks[?StackName!=`null`]|[?contains(StackName, `'$CLUSTER_NAME'-nodegroup`) == `true` || contains(StackName, `'$CLUSTER_NAME'-addon`) == `true` || contains(StackName, `'$CLUSTER_NAME'-cluster`) == `true`].StackName' | jq -r '.[]')
    for STACK_NAME in $LIST_OF_STACKS; do
           aws cloudformation delete-stack \
           --stack-name $STACK_NAME \
           --cli-read-timeout 900 \
           --cli-connect-timeout 900
        for i in `seq 1 ${NOF_RETRIES}`; do
            STACK_STATUS=$(aws cloudformation list-stacks \
            --stack-status-filter DELETE_COMPLETE \
            --no-paginate \
            --query 'StackSummaries[?StackName!=`null`]|[?contains(StackName, `'$STACK_NAME'`) == `true`].StackName' | jq -r '.|length')
            if [[ $STACK_STATUS -eq 1 ]]; then
                echo "Stack '$STACK_NAME' is deleted"
                break
            else
                echo "Stack '$STACK_NAME' is still being deleting, waiting..."
            fi
            if [[ $i == $NOF_RETRIES ]]; then
                echo "Timeout! Stack deleting could not be finished"
                return 42
            fi
            sleep 20
        done
    done
}
# The function waits until all Elastic Load Balancers attached to EC2 instances (under the current Kubernetes context) are deleted. Takes a single argument - timeout.
wait_for_elb_deleted()
{
    local TIMEOUT_IN_MINS=$1
    local NOF_RETRIES=$(( $TIMEOUT_IN_MINS * 6 ))
    INSTANCE_IDS=$(kubectl get nodes -o json | jq -r 'try([.items[].metadata.annotations."csi.volume.kubernetes.io/nodeid"][]|fromjson|."ebs.csi.aws.com"|select( . != null ))'| tr '\n' '|' | sed '$s/|$/\n/' | awk '{ print "\""$0"\""}')
    if [ ! -z "$INSTANCE_IDS" ]; then
        for i in `seq 1 ${NOF_RETRIES}`; do
            ACTIVE_ELB=$(aws elb describe-load-balancers | grep -E $INSTANCE_IDS >/dev/null; echo $?)
            if [ $ACTIVE_ELB -eq 1 ] ; then
               echo "Load Balancers are deleted."
               exit 0
            else
               echo "Load Balancers are still being deleting, waiting..."
            fi
            if [[ $i == $NOF_RETRIES ]]; then
                echo "Timeout! Deleting of Load Balancers couldn't be finished."
                return 42
            fi
            sleep 10
        done
    else
      echo "The required annotations 'csi.volume.kubernetes.io/nodeid' are missing in the EC2 instances metadata."
      exit 0
    fi
}

# This function will restart all instances that are not in ready status and wait until it will be ready
wait_for_instance_restarted()
{
   local TIMEOUT_IN_MINS=$1
   local NOF_RETRIES=$(( $TIMEOUT_IN_MINS * 3 ))
   NUMBER_NON_READY_INSTANCES=$(oc get machine -n openshift-machine-api -o json | jq -r '[.items[] | select(.status.providerStatus.instanceState | select(contains("running")|not))]|length')
   NON_READY_INSTANCE=$(oc get machines -n openshift-machine-api -o json | jq -r '[.items[] | select(.status.providerStatus.instanceState | select(contains("running")|not))]|.[].status.providerStatus.instanceId')
   if [[ ${NUMBER_NON_READY_INSTANCES} -ne 0 ]]; then
      for INSTANCE in ${NON_READY_INSTANCE}; do
         STOPPING_INSTANCE_STATE=$(aws ec2 stop-instances --instance-ids ${INSTANCE} | jq -r '.StoppingInstances[0].CurrentState.Name')
         echo "Stop instance $INSTANCE...The current instance state is $STOPPING_INSTANCE_STATE"
         aws ec2 wait instance-stopped --instance-ids ${INSTANCE}
         STARTING_INSTANCE_STATE=$(aws ec2 start-instances --instance-ids ${INSTANCE} | jq -r '.StartingInstances[0].CurrentState.Name')
         aws ec2 wait instance-running --instance-ids ${INSTANCE}
         echo "Starting instance $INSTANCE...The current instance state is $STARTING_INSTANCE_STATE"
         for i in `seq 1 ${NOF_RETRIES}`; do
            NUMBER_NON_READY_INSTANCES=$(oc get machine -n openshift-machine-api -o json | jq -r '[.items[] | select(.status.providerStatus.instanceState | select(contains("running")|not))]|length')
            if [[ ${NUMBER_NON_READY_INSTANCES} -eq 0 ]]; then
               echo "All instances are in 'Ready' status."
               return 0
            else
               echo "The instances restarted but are not ready yet. Waiting..."
            fi
            if [[ ${i} == ${NOF_RETRIES} ]]; then
               echo "Timeout! Restarted instances are still not ready."
               return 42
            fi
            sleep 20
         done
      done
   else
      echo "All instances are in 'Ready' status."
   fi
}

# The function generates test suite files which are contains list of focused tests. Takes a single argument - number of files to be generated.
# Returns the specified number of files with name test_suite_XX where XX - suffix number of file starting with '01'. The tests will be equally splitted between files.
generate_test_suites()
{
   mkdir suite_files
   local GINKGO_VERSION=v2.1.6
   make ginkgo GINKGO_VERSION=$GINKGO_VERSION
   SUITE_LIST=$(find test/e2e -type f \
     -name "*_test.go" \
   ! -name "hazelcast_backup_slow_test.go" \
   ! -name "hazelcast_wan_slow_test.go" \
   ! -name "custom_resource_test.go" \
   ! -name "client_port_forward_test.go" \
   ! -name "e2e_suite_test.go" \
   ! -name "helpers_test.go" \
   ! -name "util_test.go" \
   ! -name "options_test.go")
   for SUITE_NAME in $SUITE_LIST; do
       $(go env GOBIN)/ginkgo/$GINKGO_VERSION/ginkgo outline --format=csv "$SUITE_NAME" | grep -E "It|DescribeTable" | awk -F "\"*,\"*" '{print $2}' | awk '{ print "\""$0"\""}'| awk '{print "--focus=" $0}' | shuf >> TESTS_LIST
   done
   split --number=r/$1 TESTS_LIST suite_files/test_suite_ --numeric-suffixes=1 -a 2
   for i in $(ls suite_files/); do
       wc -l "suite_files/$i"
       cat <<< $(tr '\n' ' ' < suite_files/$i) > suite_files/$i
       cat suite_files/"$i"
   done
}

# The function merges all test reports (XML) files from each node into one report.
# Takes a single argument - the WORKFLOW_ID (kind,gke,eks, aks etc.)
merge_xml_test_reports()
{
  sudo apt-get install -y xmlstarlet
  local HZ_VERSIONS=("ee" "os")
  for hz_version in "${HZ_VERSIONS[@]}"; do
      IFS=$'\n'
      local PARENT_TEST_REPORT_FILE="${GITHUB_WORKSPACE}/allure-results/$1/test-report-$hz_version-01.xml"
      # for each test report file (except parent one) extracted the test cases
      for ALLURE_SUITE_FILE in $(find ${GITHUB_WORKSPACE}/allure-results/$1 -type f \
            -name 'test-report-'$hz_version'-?[0-9].xml' \
          ! -name 'test-report-'$hz_version'-01.xml'); do
          local TEST_CASES=$(sed '1,/<\/properties/d;/<\/testsuite/,$d' $ALLURE_SUITE_FILE)
          # insert extracted test cases into parent_test_report_file
          printf '%s\n' '0?<\/testcase>?a' $TEST_CASES . x | ex $PARENT_TEST_REPORT_FILE
      done
          #remove 'SynchronizedBeforeSuite' and 'AfterSuite' xml tags from the final report
          cat <<< $(xmlstarlet ed -d '//testcase[@name="[SynchronizedBeforeSuite]" and @status="passed"]' $PARENT_TEST_REPORT_FILE) > $PARENT_TEST_REPORT_FILE
          cat <<< $(xmlstarlet ed -d '//testcase[@name="[AfterSuite]" and @status="passed"]' $PARENT_TEST_REPORT_FILE) > $PARENT_TEST_REPORT_FILE
          # for each test name verify status
      for TEST_NAME in $(xmlstarlet sel -t -v "//testcase/@name" $PARENT_TEST_REPORT_FILE); do
          local IS_PASSED=$(xmlstarlet sel -t -v 'count(//testcase[@name="'"${TEST_NAME}"'" and @status="passed"])' $PARENT_TEST_REPORT_FILE)
          local IS_FAILED=$(xmlstarlet sel -t -v 'count(//testcase[@name="'"${TEST_NAME}"'" and @status="failed"])' $PARENT_TEST_REPORT_FILE)
          if [[ "$IS_PASSED" -ge 1 || "$IS_FAILED" -ge 1 ]]; then
          # if test is 'passed' or 'failed' then remove all tests with 'skipped' status and remove duplicated tags with 'passed' and 'failed' statuses except one
              cat <<< $(xmlstarlet ed -d '//testcase[@name="'"${TEST_NAME}"'" and @status="skipped"]' $PARENT_TEST_REPORT_FILE) > $PARENT_TEST_REPORT_FILE
              cat <<< $(xmlstarlet ed -d '(//testcase[@name="'"${TEST_NAME}"'" and @status="passed"])[position()>1]' $PARENT_TEST_REPORT_FILE) > $PARENT_TEST_REPORT_FILE
              cat <<< $(xmlstarlet ed -d '(//testcase[@name="'"${TEST_NAME}"'" and @status="failed"])[position()>1]' $PARENT_TEST_REPORT_FILE) > $PARENT_TEST_REPORT_FILE
          else
          # if tests in not in 'passed' or 'failed' statuses, then remove all duplicated tags with 'skipped' statuses except one
              cat <<< $(xmlstarlet ed -d '(//testcase[@name="'"${TEST_NAME}"'" and @status="skipped"])[position()>1]' $PARENT_TEST_REPORT_FILE) > $PARENT_TEST_REPORT_FILE
          fi
      done
      # count 'total' and 'skipped' number of tests and update the values in the final report
      local TOTAL_TESTS=$(xmlstarlet sel -t -v 'count(//testcase)' $PARENT_TEST_REPORT_FILE)
      local SKIPPED_TESTS=$(xmlstarlet sel -t -v 'count(//testcase[@status="skipped"])' $PARENT_TEST_REPORT_FILE)
      sed -i 's/tests="[^"]*/tests="'$TOTAL_TESTS'/g' $PARENT_TEST_REPORT_FILE
      sed -i 's/skipped="[^"]*/skipped="'$SKIPPED_TESTS'/g' $PARENT_TEST_REPORT_FILE
      # remove all test report files except parent one for further processing
      find ${GITHUB_WORKSPACE}/allure-results/$1 -type f -name 'test-report-'$hz_version'-?[0-9].xml' ! -name 'test-report-'$hz_version'-01.xml' -delete
  done
}

# Function clean up the final test JSON files: removes all 'steps' objects that don't contain the 'name' object, 'Text' word in the name object, and CR_ID text.
# It will also remove the 'By' annotation, converts 'Duration' time (h,m,s) into 'ms', added a URL with an error line which is the point to the source code, and finally added a direct link into the log system.
# Takes a single argument - the WORKFLOW_ID (kind,gke,eks, aks etc.)
update_test_files()
{
      local WORKFLOW_ID=$1
      local CLUSTER_NAME=$2
      local REPOSITORY_OWNER=$3
      cd allure-history/$WORKFLOW_ID/${GITHUB_RUN_NUMBER}/data/test-cases
      local GRAFANA_BASE_URL="https://hazelcastoperator.grafana.net"
      local BEGIN_TIME=$(date +%s000 -d "- 3 hours")
      local END_TIME=$(date +%s000 -d "+ 1 hours")
      for i in $(ls); do
          cat <<< $(jq -e 'del(.testStage.steps[] | select(has("name") and (.name | select(contains("CR_ID")|not)) and (.name | select(contains("Text")|not))))
                               |.testStage.steps[].name |= sub("&{Text:";"")
                               |.testStage.steps[].name |= sub("}";"")
                               |walk(if type == "object" and .steps then . | .time={"duration": .name} else . end)
                               |.testStage.steps[].name |= sub(" Duration.*";"")
                               |.testStage.steps[].time.duration |= sub(".*Duration:";"")
                               |.testStage.steps[].time.duration |= (if contains("ms") then split("ms") | .[0]|tonumber elif contains("CR_ID") then . elif contains("m") then split("m") | ((.[0]|tonumber)*60+(.[1]|.|= sub("s";"")|tonumber))*1000 elif contains("s") then split("s") | .[0]|tonumber*1000 else . end)
                               |.testStage.steps[]+={status: "passed"}
                               |(if .status=="failed" then .+={links: [.statusTrace|split("\n")
                               |to_entries
                               |walk(if type == "object" and (.value | select(contains("hazelcast-platform-operator/hazelcast-platform-operator"))) then . else . end)
                               |del(.[].key)
                               |.[].value|=sub("\\t";"")
                               |.[].value|=sub("\\+0.*";"")
                               |.[].value|=sub(" ";"")
                               |.[].value|= sub("/home/runner/work/hazelcast-platform-operator/hazelcast-platform-operator";"https://github.com/'${REPOSITORY_OWNER}'/hazelcast-platform-operator/blob/main")
                               |.[].value|= sub(".go:";".go#L")
                               |unique
                               |to_entries[]
                               |.+={name: ("ERROR_LINE"+ "_" + (.key|tonumber+1|tostring))}
                               |.url+=.value[]
                               |del(.key)|del(.value)
                               |.+={type: "issue"}]}
                               |.testStage.steps[-1]+={status: "failed"} else . end)' $i) > $i

         local NUMBER_OF_TEST_RUNS=$(jq -r '[(.testStage.steps |to_entries[]| select(.value.name | select(contains("setting the label and CR with name"))))] | length' $i)
         if [[ ${NUMBER_OF_TEST_RUNS} -gt 1 ]]; then
               local START_INDEX_OF_LAST_RETRY=$(jq -r '[(.testStage.steps |to_entries[]| select(.value.name | select(contains("setting the label and CR with name"))))][-1].key-1' $i)
               cat <<< $(jq -e 'del(.testStage.steps[0:'${START_INDEX_OF_LAST_RETRY}'])' $i) > $i
         fi
         local TEST_STATUS=$(jq -r '.status' $i)
         if [[ ${TEST_STATUS} != "skipped" ]]; then
            cat <<< $(jq -e '.extra.tags={"tag": .testStage.steps[].name | select(contains("CR_ID")) | sub("CR_ID:"; "")}|del(.testStage.steps[] | select(.name | select(contains("CR_ID"))))' $i) > $i
            local CR_ID=$(jq -r '.extra.tags.tag' $i)
            local LINK=$(echo $GRAFANA_BASE_URL\/d\/-Lz9w3p4z\/all-logs\?orgId=1\&var-cluster="$CLUSTER_NAME"\&var-cr_id="$CR_ID"\&var-text=\&from="$BEGIN_TIME"\&to="$END_TIME")
            cat <<< $(jq -e '.links|= [{"name":"LOGS","url":"'"$LINK"'",type: "tms"}] + .' $i) > $i
        fi
      done
}

#This function sync certification tags and add the latest tag to the published image
sync_certificated_image_tags()
{
     local PROJECT_ID=$1
     local CERT_IMAGE_ID=$2
     local RHEL_API_KEY=$3
     curl -X 'POST' \
     "https://catalog.redhat.com/api/containers/v1/projects/certification/id/${PROJECT_ID}/requests/images" \
     -H 'accept: application/json' \
     -H 'Content-Type: application/json' \
     -d "{
           \"image_id\": \"${CERT_IMAGE_ID}\",
           \"operation\": \"sync-tags\"
         }" \
     -H "X-API-KEY: ${RHEL_API_KEY}"
}