#!/bin/bash

set -euo pipefail

function print_checkpoint {
    echo
    bold_blue "==================================  $@"
}

function bold_red {
    echo -e "\e[1;31m$1\e[0m"
}

function bold_green {
    echo -e "\e[1;32m$1\e[0m"
}

function bold_blue {
    echo -e "\e[1;34m$1\e[0m"
}

function setup {
    print_checkpoint "Setting up"
    : ${TEST_APP_NAME:=log-cache-cli-integration}
    cf target -o ${CF_ORG:?Need to provide a CF organization} -s ${CF_SPACE:?Need to provide a CF space}

    cd $(dirname $0)

    NO_STANDALONE_LC=true ./install.sh
}

function cleanup_test_app {
    print_checkpoint "Cleaning up"

    rm -rf ${TEST_APP_DIR}
    if [[ -n ${TEST_APP_GUID:-} ]]; then
        cf delete -f -r ${TEST_APP_NAME}
    fi
}

function push_test_app {
    print_checkpoint "Pushing test app"
    TEST_APP_DIR=$(mktemp -d)
    pushd $TEST_APP_DIR > /dev/null
        touch Staticfile
        cf push ${TEST_APP_NAME} \
            --random-route

        TEST_APP_GUID=$(cf app ${TEST_APP_NAME} --guid)
        TEST_APP_ROUTE=$(cf app ${TEST_APP_NAME} | awk '/route/ { print $2 }')
    popd > /dev/null
    trap "cleanup_test_app" EXIT
}

function create_test_metrics {
    print_checkpoint "hitting app to create logs/metrics"
    curl ${TEST_APP_ROUTE} > /dev/null
    sleep 5s
}

function run_test {
    local description=$1
    local test_function=$2

    print_checkpoint "Testing ${description}"
    if (set -x; ${test_function}); then
        bold_green "OK"
    else
        bold_red "FAILED"
    fi
}

function test_log_meta {
    cf log-meta --guid | grep ${TEST_APP_GUID}
}

function test_tail {
    cf tail -n 1000 ${TEST_APP_NAME} | grep ${TEST_APP_GUID}
}

function test_query {
    [[ $(cf query 'http{source_id="'${TEST_APP_GUID}'"}' | jq '.data.result | length') > 0 ]]
    [[ $(cf query 'http{source_id="'${TEST_APP_NAME}'"}' | jq '.data.result | length') > 0 ]]
}

function main {
    setup
    push_test_app
    create_test_metrics

    run_test "cf log-meta" test_log_meta
    run_test "cf tail" test_tail
    run_test "cf query" test_query
}

main
