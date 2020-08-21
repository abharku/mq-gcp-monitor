

# mq-gcp-monitor
This repo cotains a modified version of https://github.com/ibm-messaging/mq-metric-samples

mq-metrics-samples is a prerequisite for this project and before you try and build this package make sure you can build mq-metrics-samples and it's dependencies

It uses GCP custom metrics code to send IBM MQ metrics directly to GCP Monitoring (Stackdriver) - https://github.com/GoogleCloudPlatform/golang-samples/tree/master/monitoring/custommetric


# Build

To build cd intpo root folder and setup your GOROOT and GOPATH

For example:

export GOROOT=/usr/local/go
export GOPATH=/var/mq-gcp-monitor

Then build with below command:

go build -o $GOPATH/bin/mq_gcp ./src/github.com/ibm-messaging/mq-metric-samples/cmd/mq_gcp/*.go

This will create a go executable in bin folder.

# Run

Update the yaml ($GOPATH/mq_gcp.yaml) file with your Queue manager and Queues/channels to monitor

The yaml file has two GCP specific properties:

projectId = Id of the project where your QM resides
zone = The zone this VM/container is deployed in

# GCP authentication

Authentication is based on service account principle and it assumes the VM has monitoring roles to publish custom metrics

