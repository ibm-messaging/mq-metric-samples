#!/usr/bin/env bash

# Set variables
IBM_ENTITLEMENT_KEY=${IBM_ENTITLEMENT_KEY}

SEALED_SECRET_NAMESPACE=${SEALED_SECRET_NAMESPACE:-sealed-secrets}
SEALED_SECRET_CONTOLLER_NAME=${SEALED_SECRET_CONTOLLER_NAME:-sealed-secrets}

# Create Kubernetes Secret yaml
oc create secret generic mq-metric-samples-qm-credentials \
--from-literal=IBMMQ_CONNECTION_USER='' \
--from-literal=IBMMQ_CONNECTION_PASSWORD='' \
--dry-run=client -o yaml > delete-mq-metric-samples-qm-credentials-secret.yaml

# Encrypt the secret using kubeseal and private key from the cluster
kubeseal --scope cluster-wide --controller-name=${SEALED_SECRET_CONTOLLER_NAME} --controller-namespace=${SEALED_SECRET_NAMESPACE} -o yaml < delete-mq-metric-samples-qm-credentials-secret.yaml > mq-metric-samples-qm-credentials-secret.yaml 

# NOTE, do not check delete-ibm-entitled-key-secret.yaml into git!
rm delete-mq-metric-samples-qm-credentials-secret.yaml