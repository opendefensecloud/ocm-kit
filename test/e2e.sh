#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

DOCKER="${DOCKER:-docker}"
OCM="${OCM:-ocm}"
GO="${GO:-go}"

VERSION="${VERSION:-"0.0.$(date +%s)"}"
KEEP_ZOT=false

# Parse arguments
while [[ $# -gt 0 ]]; do
	case $1 in
    -h|--help)
      echo "e2e.sh - runs e2e tests"
      echo " "
      echo "./e2e.sh [options]"
      echo " "
      echo "options:"
      echo "-h, --help           show brief help"
      echo "--keep-zot           keeps zot running after running tests"
      echo "--version VERSION    specify component version created during test"
      exit 0
      ;;
		--keep-zot)
			KEEP_ZOT=true
			shift
			;;
		--version)
			VERSION="$2"
			shift 2
			;;
		*)
			shift
			;;
	esac
done

# Check if zot is already running
if ! ${DOCKER} ps | grep -q zot-registry; then
	echo "Starting zot registry..."
	${DOCKER} run -d -p 5000:5000 \
		  --name zot-registry \
		  -v "${SCRIPT_DIR}/fixtures/zot-config.json:/etc/zot/config.json:ro" \
		  -v zot-data:/var/lib/registry \
		  ghcr.io/project-zot/zot:v2.1.10
else
	echo "zot registry already running"
fi

# Check if CTF needs to be created and transferred
CTF_DIR="${SCRIPT_DIR}/fixtures/arc/ctf"
ARTIFACT_INDEX="${CTF_DIR}/artifact-index.json"

if [ ! -f "$ARTIFACT_INDEX" ] || ! grep -q "\"tag\":\"${VERSION}\"" "$ARTIFACT_INDEX"; then
	echo "Creating and transferring component version ${VERSION}..."
	rm -rf "${CTF_DIR}"
	(cd "$SCRIPT_DIR/fixtures/arc" && ${OCM} add componentversion --version "${VERSION}" --create --file ./ctf component-constructor.yaml)
	${OCM} transfer ctf --copy-resources "${CTF_DIR}" http://localhost:5000/my-components
else
	echo "Component version ${VERSION} already exists in CTF"
fi

echo "Running ocm-kit CLI with component version ${VERSION}..."

# Test 1: Render with default template
echo "Test 1: Rendering default Helm values template..."
OUTPUT1=$(${GO} run cmd/ocm-kit/main.go "http://localhost:5000/my-components//opendefense.cloud/arc:${VERSION}" -r helm-chart)
if echo "$OUTPUT1" | grep -q "apiserver:" && \
   echo "$OUTPUT1" | grep -q "controller:" && \
   echo "$OUTPUT1" | grep -q "etcd:" && \
   echo "$OUTPUT1" | grep -q "localhost:5000/my-components/opendefensecloud/arc-apiserver" && \
   echo "$OUTPUT1" | grep -q "localhost:5000/my-components/opendefensecloud/arc-controller-manager" && \
   echo "$OUTPUT1" | grep -q "localhost:5000/my-components/coreos/etcd"; then
	echo "✓ Test 1 passed: Default template rendered correctly"
else
	echo "✗ Test 1 failed: Default template output missing expected content"
	echo "Output was:"
	echo "$OUTPUT1"
	exit 1
fi

# Test 2: Render with override template
echo "Test 2: Rendering override Helm values template..."
OUTPUT2=$(${GO} run cmd/ocm-kit/main.go "http://localhost:5000/my-components//opendefense.cloud/arc:${VERSION}" -r helm-chart --local-helm-values-template "$SCRIPT_DIR/fixtures/arc/override-values.yaml.tpl")
if echo "$OUTPUT2" | grep -q "foobar:" && \
   echo "$OUTPUT2" | grep -q "fizzbuzz:" && \
   echo "$OUTPUT2" | grep -q "helloworld:" && \
   echo "$OUTPUT2" | grep -q "localhost:5000/my-components/opendefensecloud/arc-apiserver" && \
   echo "$OUTPUT2" | grep -q "localhost:5000/my-components/opendefensecloud/arc-controller-manager" && \
   echo "$OUTPUT2" | grep -q "localhost:5000/my-components/coreos/etcd"; then
	echo "✓ Test 2 passed: Override template rendered correctly"
else
	echo "✗ Test 2 failed: Override template output missing expected content"
	echo "Output was:"
	echo "$OUTPUT2"
	exit 1
fi

# Cleanup only if --keep-zot was not provided
if [ "$KEEP_ZOT" = false ]; then
	echo "Stopping zot registry..."
	${DOCKER} stop zot-registry
	${DOCKER} rm -f zot-registry
	${DOCKER} volume rm zot-data
else
	echo "Keeping zot registry running (--keep-zot flag provided)"
fi
