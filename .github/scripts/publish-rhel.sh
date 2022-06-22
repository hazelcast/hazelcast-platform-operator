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

publish_the_image()
{
    local PROJECT_ID=$1
    local VERSION=$2
    local RHEL_API_KEY=$3

    local IS_PUBLISHED=$(get_image published "${PROJECT_ID}" "${VERSION}" "${RHEL_API_KEY}" | jq -r '.total')
    if [[ $IS_PUBLISHED == "1" ]]; then
        echo "Image is already published, exiting"
        return 0
    fi

    local IMAGE=$(get_image not_published "${PROJECT_ID}" "${VERSION}" "${RHEL_API_KEY}")
    local IMAGE_EXISTS=$(echo $IMAGE | jq -r '.total')
    if [[ $IMAGE_EXISTS == "1" ]]; then
        local SCAN_STATUS=$(echo $IMAGE | jq -r '.data[0].scan_status')
        if [[ $SCAN_STATUS != "passed" ]]; then
            echo "Image you are trying to publish did not pass the certification test, its status is \"${SCAN_STATUS}\""
            return 1
        fi
    else
        echo "Image you are trying to publish does not exist."
        return 1
    fi

    local IMAGE_ID=$(echo "$IMAGE" | jq -r '.data[0]._id')

    # Publish the image
    echo "Publishing the image..."
    RESPONSE=$( \
        curl --silent \
            --request POST \
            --header "X-API-KEY: ${RHEL_API_KEY}" \
            --header 'Cache-Control: no-cache' \
            --header 'Content-Type: application/json' \
            --data "{\"image_id\":\"${IMAGE_ID}\" , \"tag\" : \"${VERSION}\" }" \
            "https://catalog.redhat.com/api/containers/v1/projects/certification/id/${PROJECT_ID}/requests/tags")

    echo "Created a tag request, please check if the image is published."
}

wait_for_container_publish()
{
    local PROJECT_ID=$1
    local VERSION=$2
    local RHEL_API_KEY=$3
    local TIMEOUT_IN_MINS=$4

    local NOF_RETRIES=$(( $TIMEOUT_IN_MINS / 2 ))
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
        sleep 120
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