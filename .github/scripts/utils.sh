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
# This function will restart all nodes that are not in ready status and wait until it will be ready
wait_for_node_restarted()
{
   local TIMEOUT_IN_MINS=$1
   local NOF_RETRIES=$(( $TIMEOUT_IN_MINS * 3 ))
   NON_READY_NODES=$(oc get nodes -o json | jq -r 'del(.items[].status.conditions[] | select(.type | select(contains("Ready")|not))) | .items[]| select(.status.conditions[].status | select(contains("Unknown"))).metadata.name'| wc -l)
   if [[ ${NON_READY_NODES} -ne 0 ]]; then
      for NODE in ${NON_READY_NODES}; do
         echo "Restarting node $NODE..."
         nohup oc debug node/${NODE} -T -- chroot /host sh -c "systemctl reboot" &
         sleep 20
      done
   fi
   for i in `seq 1 ${NOF_RETRIES}`; do
        NON_READY_NODES=$(oc get nodes -o json | jq -r 'del(.items[].status.conditions[] | select(.type | select(contains("Ready")|not))) | .items[]| select(.status.conditions[].status | select(contains("Unknown"))).metadata.name'| wc -l)
        if [[ ${NON_READY_NODES} -eq 0 ]]; then
           echo "All nodes are in 'Ready' status."
           return 0
        else
           echo "The nodes restarted but are not ready yet. Waiting..."
        fi
        if [[ ${i} == ${NOF_RETRIES} ]]; then
           echo "Timeout! Restarted nodes are still not ready."
           return 42
        fi
        sleep 20
   done
}