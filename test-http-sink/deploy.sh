#!/bin/bash
#
# Mock HTTP Sink Deployment Helper
#

set -e

PROJECT_ID="akomar-server-telemetry-poc"
IMAGE_NAME="test-http-sink"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

function print_usage() {
    echo "Usage: ./deploy.sh [command]"
    echo ""
    echo "Commands:"
    echo "  build       - Build Docker image"
    echo "  push        - Push image to GCR"
    echo "  deploy      - Deploy to GKE"
    echo "  ip          - Get external LoadBalancer IP"
    echo "  logs        - Follow deployment logs"
    echo "  point       - Point publisher to this sink"
    echo "  restore     - Restore publisher to staging endpoint"
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
    kubectl apply -f ../kubernetes/test-http-sink-deploy.yaml
    echo -e "${GREEN}✓ Deployment applied${NC}"
    echo ""
    echo -e "${YELLOW}Waiting for LoadBalancer IP (this takes 2-3 minutes)...${NC}"
    kubectl wait --for=condition=ready pod -l component=test-http-sink --timeout=60s
    echo -e "${GREEN}✓ Pod ready${NC}"
    echo ""
    get_ip
}

function get_ip() {
    echo -e "${GREEN}Getting LoadBalancer external IP...${NC}"
    SINK_IP=$(kubectl get service test-http-sink -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null)
    if [ -z "$SINK_IP" ]; then
        echo -e "${YELLOW}LoadBalancer IP not ready yet. Waiting...${NC}"
        kubectl get service test-http-sink -w &
        sleep 60
        SINK_IP=$(kubectl get service test-http-sink -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
    fi
    echo ""
    echo -e "${GREEN}✓ Mock sink endpoint: http://${SINK_IP}${NC}"
    echo ""
    echo "Test it:"
    echo "  curl http://${SINK_IP}/__heartbeat__"
    echo ""
    echo "Point publisher to this endpoint:"
    echo "  ./deploy.sh point"
}

function show_logs() {
    echo -e "${GREEN}Following logs (Ctrl+C to stop)...${NC}"
    kubectl logs -f deployment/test-http-sink
}

function point_publisher() {
    SINK_IP=$(kubectl get service test-http-sink -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
    if [ -z "$SINK_IP" ]; then
        echo -e "${YELLOW}Error: LoadBalancer IP not available yet${NC}"
        exit 1
    fi

    echo -e "${GREEN}Pointing publisher to mock sink: http://${SINK_IP}/submit${NC}"
    kubectl patch configmap test-go-http-publisher-config -p "{\"data\":{\"ENDPOINT\":\"http://${SINK_IP}/submit\"}}"

    echo -e "${YELLOW}Restart publisher to apply changes? (y/n)${NC}"
    read -p "" restart
    if [ "$restart" = "y" ]; then
        kubectl rollout restart deployment/test-go-http-publisher
        echo -e "${GREEN}✓ Publisher restarted${NC}"
    fi
}

function restore_publisher() {
    echo -e "${GREEN}Restoring publisher to staging endpoint${NC}"
    kubectl patch configmap test-go-http-publisher-config -p '{"data":{"ENDPOINT":"https://stage.ingestion-edge.nonprod.dataops.mozgcp.net/submit"}}'

    echo -e "${YELLOW}Restart publisher? (y/n)${NC}"
    read -p "" restart
    if [ "$restart" = "y" ]; then
        kubectl rollout restart deployment/test-go-http-publisher
        echo -e "${GREEN}✓ Publisher restored to staging${NC}"
    fi
}

function cleanup() {
    echo -e "${YELLOW}This will delete the mock sink. Continue? (y/n)${NC}"
    read -p "" confirm
    if [ "$confirm" = "y" ]; then
        echo -e "${GREEN}Cleaning up...${NC}"
        kubectl delete -f ../kubernetes/test-http-sink-deploy.yaml
        echo -e "${GREEN}✓ Cleanup complete${NC}"
    fi
}

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
    ip)
        get_ip
        ;;
    logs)
        show_logs
        ;;
    point)
        point_publisher
        ;;
    restore)
        restore_publisher
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
