#!/usr/bin/env bash
export DOCKERIMAGE=${DOCKERIMAGE:-syncano/codebox}
export APP=codebox
export VERSION="$2"

TARGET="$1"
LB_TOTAL_NUM=1

usage() { echo "* Usage: $0 <environment> <version> [--skip-push]" >&2; exit 1; }
[[ -n $TARGET ]] || usage
[[ -n $VERSION ]] || usage

set -euo pipefail

if ! command -v kubectl > /dev/null; then
    echo "! kubectl not installed" >&2; exit 1
fi

if [[ ! -f "deploy/env/${TARGET}.env" ]]; then
    echo "! environment ${TARGET} does not exist in deploy/env/"; exit 1
fi

# Parse arguments.
PUSH=true
for PARAM in "${@:3}"; do
    case $PARAM in
        --skip-push)
          PUSH=false
          ;;
        *)
          usage
          ;;
    esac
done

envsubst() {
    for var in $(compgen -e); do
        echo "$var: \"${!var//\"/\\\"}\""
    done | PYTHONWARNINGS=ignore jinja2 "$1"
}

deploy_broker() {
    # Deploy broker.
    LB_ADDRS=$(seq -s, -f "codebox-lb-%02g.default:9000" 1 $LB_TOTAL_NUM)
    export LB_ADDRS=${LB_ADDRS%,}
    REPLICAS=$(kubectl get deployment/codebox-broker -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "${BROKER_MIN}")
    export REPLICAS
    echo "* Deploying Broker for LB=${LB_ADDRS}, replicas=${REPLICAS}."
    envsubst deploy/yaml/broker-deployment.yml.j2 | kubectl apply -f -
    envsubst deploy/yaml/broker-hpa.yml.j2 | kubectl apply -f -

    echo "* Deploying Service for Broker."
    envsubst deploy/yaml/broker-service.yml.j2 | kubectl apply -f -

    kubectl rollout status deployment/codebox-broker
}


echo "* Starting deployment for $TARGET at $VERSION."

# Setup environment variables.
set -a
# shellcheck disable=SC1090
source deploy/env/"${TARGET}".env
set +a
BUILDTIME=$(date +%Y-%m-%dt%H%M)
export BUILDTIME


# Push docker image.
if $PUSH; then
	echo "* Tagging $DOCKERIMAGE $VERSION."
	docker tag "$DOCKERIMAGE" "$DOCKERIMAGE":"$VERSION"

	echo "* Pushing $DOCKERIMAGE:$VERSION."
	docker push "$DOCKERIMAGE":"$VERSION"
fi

IMAGE="$DOCKERIMAGE":"$VERSION"
export IMAGE


# Create configmap.
echo "* Updating ConfigMap."
CONFIGMAP="apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: ${APP}\ndata:\n"
while read -r line; do
    if [[ -n "${line}" && "${line}" != "#"* ]]; then
        CONFIGMAP+="  ${line%%=*}: \"${line#*=}\"\n"
    fi
done < deploy/env/"${TARGET}".env
echo -e "$CONFIGMAP" | kubectl apply -f -


# Create secrets.
echo "* Updating Secrets."
SECRETS="apiVersion: v1\nkind: Secret\nmetadata:\n  name: ${APP}\ntype: Opaque\ndata:\n"
while read -r line; do
    if [[ -n "${line}" && "${line}" != "#"* ]]; then
        SECRETS+="  ${line%%=*}: $(echo -n "${line#*=}" | base64 | tr -d '\n')\n"
    fi
done < deploy/env/"${TARGET}".secrets.unenc
echo -e "$SECRETS" | kubectl apply -f -


# Deploy broker first if we will downscale amount of LBs.
OLD_LB_TOTAL_NUM=$(( $(kubectl get deploy -l app=codebox,type=lb | wc -l) - 1 ))
[ $LB_TOTAL_NUM -lt $OLD_LB_TOTAL_NUM ] && deploy_broker


# Deploy worker startup daemonset.
kubectl create configmap codebox-dind --from-file=deploy/dind-run.sh -o yaml --dry-run | kubectl apply -f -
kubectl create configmap codebox-startup --from-file=deploy/worker-setup.sh -o yaml --dry-run | kubectl apply -f -
envsubst deploy/yaml/worker-daemonset.yml.j2 | kubectl apply -f -
echo ". Waiting for Worker Docker Daemonset deployment to finish..."
kubectl rollout status daemonset/codebox-docker

# Deploy LB RBAC.
envsubst deploy/yaml/lb-rbac.yml.j2 | kubectl apply -f -

# Start with deployment of LB-workers pairs.
for (( LB_NUM=1; LB_NUM <= LB_TOTAL_NUM; LB_NUM++ )); do
    LB_NUM=$(printf "%02d" $LB_NUM)
    export LB_NUM

    echo "* Deploying LB-${LB_NUM}."
    envsubst deploy/yaml/lb-deployment.yml.j2 | kubectl apply -f -

    echo "* Deploying Internal Service for LB-${LB_NUM}."
    envsubst deploy/yaml/lb-internal-service.yml.j2 | kubectl apply -f -

    echo "* Deploying Service for LB-${LB_NUM}."
    envsubst deploy/yaml/lb-service.yml.j2 | kubectl apply -f -


    # Wait for new LB IP address
    echo "* Getting new LB IP address."
    PODNAME=$(kubectl get pods -l name=codebox-lb-"${LB_NUM}" -l buildtime="${BUILDTIME}" -o name | tail -n1)
    for i in {1..600}; do
        LB_IP=$(kubectl get "${PODNAME}" -o jsonpath='{.status.podIP}')
        [[ -z $LB_IP ]] || break
        sleep 1
        echo "! Failed getting new LB IP - retrying... #$i"
    done

    if [[ -z $LB_IP ]]; then
        echo "! Couldn't get load balancer IP address!"
        exit 1
    fi


    # Deploy workers.
    LB_ADDR=codebox-lb-internal-${LB_NUM}.default:9000
    export LB_ADDR
    REPLICAS=$(kubectl get deployment/codebox-worker-"${LB_NUM}" -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "${SCALING_MIN}")
    export REPLICAS
    echo "* Deploying Worker for LB-${LB_NUM} with IP=${LB_ADDR}, replicas=${REPLICAS}."
    envsubst deploy/yaml/worker-deployment.yml.j2 | kubectl apply -f -


    # Wait for deployment to finish.
    echo
    echo ". Waiting for Worker-${LB_NUM} deployment to finish..."
    kubectl rollout status deployment/codebox-worker-"${LB_NUM}"

    echo ". Waiting for LB-${LB_NUM} deployment to finish..."
    kubectl rollout status deployment/codebox-lb-"${LB_NUM}"
    echo
done

envsubst deploy/yaml/worker-service.yml.j2 | kubectl apply -f -
envsubst deploy/yaml/lb-pdb.yml.j2 | kubectl apply -f -
envsubst deploy/yaml/lb-discovery-service.yml.j2 | kubectl apply -f -


# Deploy broker last if we will upscale amount of LBs.
[ $LB_TOTAL_NUM -ge $OLD_LB_TOTAL_NUM ] && deploy_broker
