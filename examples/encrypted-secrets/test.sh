#!/usr/bin/env bash
# test.sh - Test script for the encrypted secrets example
#
# This script demonstrates running the disco agent with encrypted secrets enabled.
# It will run in one-shot mode and output to a local file for inspection.

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Encrypted Secrets Example Test ===${NC}\n"

echo -e "${GREEN}Testing agent with Kubernetes secrets${NC}"
echo ""

# Enable encrypted secrets
export ARK_SEND_SECRETS="true"

# Check Kubernetes connectivity
if ! kubectl cluster-info &> /dev/null; then
    echo -e "${RED}Error: Unable to connect to Kubernetes cluster${NC}"
    echo "Please ensure your kubeconfig is configured correctly."
    exit 1
fi

echo -e "${GREEN}✓ Connected to Kubernetes cluster${NC}"
CONTEXT=$(kubectl config current-context)
echo "  Context: ${CONTEXT}"
echo ""

# Check for secrets
SECRET_COUNT=$(kubectl get secrets --all-namespaces --no-headers 2>/dev/null | wc -l | tr -d ' ')
echo "Found ${SECRET_COUNT} secrets in cluster"
echo ""

# Run the agent in one-shot mode with output to file
OUTPUT_FILE="output.json"
echo -e "${GREEN}Running disco agent with encrypted secrets enabled...${NC}"
echo "Command: go run ../.. agent --agent-config-file config.yaml --one-shot --output-path ${OUTPUT_FILE}"
echo ""

if go run ../.. agent \
    --agent-config-file config.yaml \
    --one-shot \
    --output-path "${OUTPUT_FILE}"; then

    echo ""
    echo -e "${GREEN}✓ Agent completed successfully${NC}"

    # Check if output file was created
    if [ -f "${OUTPUT_FILE}" ]; then
        echo -e "${GREEN}✓ Output file created: ${OUTPUT_FILE}${NC}"
    else
        echo -e "${RED}✗ Output file was not created${NC}"
        exit 1
    fi
else
    echo ""
    echo -e "${RED}✗ Agent failed${NC}"
    exit 1
fi
