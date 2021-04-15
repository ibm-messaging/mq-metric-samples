# Monitoring metrics in Red Hat OpenShift
This directory contains sample files that demonstrate how to deploy the Prometheus
monitor to gather metrics in a Red Hat OpenShift / Cloud Pak for Integration deployment.

The basic steps to do this are as follows;

```
# Build the Prometheus monitor container as normal
cd mq-metric-samples/scripts
./buildRuntime.sh mq_prometheus

# Tag and push the Docker image to the container registry used by your OpenShift cluster
docker tag mq-metric-prometheus:5.2.0 your.repo/your-namespace/mq-metric-prometheus:1.0
docker push your.repo/your-namespace/mq-metric-prometheus:1.0

# Create a ConfigMap and Secret to configure the settings that you wish to apply to your monitor.
# Update these to suit your requirements
oc create configmap metrics-configuration \
    --from-literal=IBMMQ_CONNECTION_QUEUEMANAGER='QM1' \
    --from-literal=IBMMQ_CONNECTION_CONNNAME='quickstart-cp4i-ibm-mq(1414)' \
    --from-literal=IBMMQ_CONNECTION_CHANNEL='SYSTEM.DEF.SVRCONN' \
    --from-literal=IBMMQ_OBJECTS_QUEUES='*,!SYSTEM.*,!AMQ.*' \
    --from-literal=IBMMQ_OBJECTS_SUBSCRIPTIONS='!$SYS*' \
    --from-literal=IBMMQ_OBJECTS_TOPICS='!*' \
    --from-literal=IBMMQ_GLOBAL_USEPUBLICATIONS=false \
    --from-literal=IBMMQ_GLOBAL_USEOBJECTSTATUS=true \
    --from-literal=IBMMQ_GLOBAL_CONFIGURATIONFILE='' \
    --from-literal=IBMMQ_GLOBAL_LOGLEVEL=INFO

oc create secret generic metrics-credentials \
    --from-literal=IBMMQ_CONNECTION_USER='dummyuser' \
    --from-literal=IBMMQ_CONNECTION_PASSWORD='dummypassword'



# Use the samples in this directory to deploy the Prometheus container to OpenShift
cd mq-metric-samples/openshift

# Create a new ServiceAccount that will ensure the metrics pod is
# deployed using the most secure Restricted SCC
oc apply -f sa-pod-deployer.yaml

# Update the spec.containers.image attribute in metrics-pod.yaml to match
# your container registry and image name using a text editor
vi metrics-pod.yaml

# Deploy the metrics pod using the service account
oc apply -f ./metrics-pod.yaml --as=my-service-account

# Create a Service object that exposes the metrics pod so that it can
# be discovered by monitoring tools that are looking for Prometheus endpoints
#
# Note that the spec.selector.app matches the metadata.labels.app property
# defined in metrics-pod.yaml
oc apply -f ./metrics-service.yaml
```


Once the metrics pod is deployed you can see it in action by looking at the logs
for the pod, and the "IBMMQ Collect" statements showing that the metrics are being
scraped by the Prometheus agent running in your OpenShift cluster.
```
oc logs mq-metric-prometheus


IBM MQ metrics exporter for Prometheus monitoring
Build         : 20210410-173628 
Commit Level  : 3dd2c0d
Build Platform: Darwin/

time="2021-04-13T20:12:52Z" level=info msg="Trying to connect as client using ConnName: quickstart-cp4i-ibm-mq(1414), Channel: SYSTEM.DEF.SVRCONN"
time="2021-04-13T20:12:52Z" level=info msg="Connected to queue manager  QM1"
time="2021-04-13T20:12:52Z" level=info msg="IBMMQ Describe started"
time="2021-04-13T20:12:52Z" level=info msg="Platform is UNIX"
time="2021-04-13T20:12:52Z" level=info msg="Listening on http address :9157"
time="2021-04-13T20:12:55Z" level=info msg="IBMMQ Collect started 14000001720300"
time="2021-04-13T20:12:55Z" level=info msg="Collection time = 0 secs"
time="2021-04-13T20:13:55Z" level=info msg="IBMMQ Collect started 14000003035700"
time="2021-04-13T20:13:55Z" level=info msg="Collection time = 0 secs"
```