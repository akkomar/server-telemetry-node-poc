#!/bin/bash
#
# GKE Deployment Helper Script for test-go-http-publisher
# Usage: ./deploy.sh [command]
#

set -e

PROJECT_ID="akomar-server-telemetry-poc"
IMAGE_NAME="test-go-http-publisher"
DEPLOYMENT_NAME="test-go-http-publisher"
CONFIGMAP_NAME="test-go-http-publisher-config"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

function print_usage() {
    echo "Usage: ./deploy.sh [command]"
    echo ""
    echo "Commands:"
    echo "  build       - Build Docker image"
    echo "  push        - Push image to GCR"
    echo "  deploy      - Deploy to GKE"
    echo "  restart     - Restart deployment (pick up new config)"
    echo "  logs        - Follow deployment logs"
    echo "  status      - Show pod status and resource usage"
    echo "  config      - Update ConfigMap (interactive)"
    echo "  test-N      - Run test N from testing guide"
    echo "  cleanup     - Delete deployment from GKE"
    echo "  all         - Build, push, and deploy"
    echo ""
}

function build_image() {
    echo -e "${GREEN}Building Docker image...${NC}"
    docker build --platform linux/amd64 -t ${IMAGE_NAME} .
    echo -e "${GREEN}✓ Build complete${NC}"
}

function push_image() {
    echo -e "${GREEN}Pushing image to GCR...${NC}"
    docker tag ${IMAGE_NAME} gcr.io/${PROJECT_ID}/${IMAGE_NAME}
    docker push gcr.io/${PROJECT_ID}/${IMAGE_NAME}
    echo -e "${GREEN}✓ Push complete${NC}"
}

function deploy_to_gke() {
    echo -e "${GREEN}Deploying to GKE...${NC}"
    kubectl apply -f ../kubernetes/test-go-http-publisher-deploy.yaml
    echo -e "${GREEN}✓ Deployment applied${NC}"
    echo ""
    echo "Wait for pod to start..."
    kubectl wait --for=condition=ready pod -l component=${DEPLOYMENT_NAME} --timeout=60s
    echo -e "${GREEN}✓ Pod ready${NC}"
}

function restart_deployment() {
    echo -e "${GREEN}Restarting deployment...${NC}"
    kubectl rollout restart deployment/${DEPLOYMENT_NAME}
    kubectl rollout status deployment/${DEPLOYMENT_NAME}
    echo -e "${GREEN}✓ Deployment restarted${NC}"
}

function show_logs() {
    echo -e "${GREEN}Following logs (Ctrl+C to stop)...${NC}"
    kubectl logs -f deployment/${DEPLOYMENT_NAME}
}

function show_status() {
    echo -e "${GREEN}=== Pod Status ===${NC}"
    kubectl get pods -l component=${DEPLOYMENT_NAME}
    echo ""

    echo -e "${GREEN}=== Resource Usage ===${NC}"
    kubectl top pod -l component=${DEPLOYMENT_NAME}
    echo ""

    echo -e "${GREEN}=== Recent Logs ===${NC}"
    kubectl logs deployment/${DEPLOYMENT_NAME} --tail=20
}

function update_config() {
    echo -e "${YELLOW}Current configuration:${NC}"
    kubectl get configmap ${CONFIGMAP_NAME} -o yaml | grep -A 10 "^data:"
    echo ""

    read -p "Events per second (EVENTS_PER_SEC): " rate
    read -p "Concurrency (CONCURRENCY): " concurrency

    echo -e "${GREEN}Updating ConfigMap...${NC}"
    kubectl patch configmap ${CONFIGMAP_NAME} -p "{\"data\":{\"EVENTS_PER_SEC\":\"${rate}\",\"CONCURRENCY\":\"${concurrency}\"}}"

    echo -e "${YELLOW}Restart deployment to apply changes? (y/n)${NC}"
    read -p "" restart
    if [ "$restart" = "y" ]; then
        restart_deployment
    fi
}

function run_test() {
    test_num=$1
    case $test_num in
        1)
            echo -e "${GREEN}Running Test 1: Baseline${NC}"
            kubectl patch configmap ${CONFIGMAP_NAME} -p '{"data":{"EVENTS_PER_SEC":"100","CONCURRENCY":"20"}}'
            ;;
        2)
            echo -e "${GREEN}Running Test 2: Low Load${NC}"
            kubectl patch configmap ${CONFIGMAP_NAME} -p '{"data":{"EVENTS_PER_SEC":"500","CONCURRENCY":"50"}}'
            ;;
        3)
            echo -e "${GREEN}Running Test 3: Medium Load${NC}"
            kubectl patch configmap ${CONFIGMAP_NAME} -p '{"data":{"EVENTS_PER_SEC":"1000","CONCURRENCY":"100"}}'
            ;;
        4)
            echo -e "${GREEN}Running Test 4: Higher Concurrency${NC}"
            kubectl patch configmap ${CONFIGMAP_NAME} -p '{"data":{"EVENTS_PER_SEC":"2000","CONCURRENCY":"200"}}'
            ;;
        5)
            echo -e "${GREEN}Running Test 5: High Concurrency${NC}"
            kubectl patch configmap ${CONFIGMAP_NAME} -p '{"data":{"EVENTS_PER_SEC":"5000","CONCURRENCY":"500"}}'
            ;;
        6)
            echo -e "${GREEN}Running Test 6: Very High Concurrency${NC}"
            kubectl patch configmap ${CONFIGMAP_NAME} -p '{"data":{"EVENTS_PER_SEC":"10000","CONCURRENCY":"1000"}}'
            ;;
        7)
            echo -e "${GREEN}Running Test 7: Maximum Stress Test${NC}"
            kubectl patch configmap ${CONFIGMAP_NAME} -p '{"data":{"EVENTS_PER_SEC":"20000","CONCURRENCY":"2000"}}'
            ;;
        *)
            echo -e "${RED}Invalid test number. Choose 1-7${NC}"
            exit 1
            ;;
    esac

    echo -e "${YELLOW}Restarting deployment...${NC}"
    restart_deployment

    echo ""
    echo -e "${GREEN}Test ${test_num} started. Monitor with:${NC}"
    echo "  ./deploy.sh logs"
    echo "  ./deploy.sh status"
}

function cleanup() {
    echo -e "${YELLOW}This will delete the deployment. Continue? (y/n)${NC}"
    read -p "" confirm
    if [ "$confirm" = "y" ]; then
        echo -e "${GREEN}Cleaning up...${NC}"
        kubectl delete -f ../kubernetes/test-go-http-publisher-deploy.yaml
        echo -e "${GREEN}✓ Cleanup complete${NC}"
    fi
}

# Main command routing
case "${1}" in
    build)
        build_image
        ;;
    push)
        push_image
        ;;
    deploy)
        deploy_to_gke
        ;;
    restart)
        restart_deployment
        ;;
    logs)
        show_logs
        ;;
    status)
        show_status
        ;;
    config)
        update_config
        ;;
    test-1|test-2|test-3|test-4|test-5|test-6|test-7)
        test_num="${1#test-}"
        run_test $test_num
        ;;
    cleanup)
        cleanup
        ;;
    all)
        build_image
        push_image
        deploy_to_gke
        ;;
    *)
        print_usage
        exit 1
        ;;
esac
