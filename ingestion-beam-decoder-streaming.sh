#!/bin/bash
set -eo pipefail

# This script assumes it's being run from the ingestion-beam directory of the gcp-ingestion repo
# with the code set to current contents of the main branch.

PROJECT="akomar-server-telemetry-poc"
JOB_NAME="server-events-decoder-streaming"

set -x

#   clean \
#   compile \
mvn -pl ingestion-beam -am \
  exec:java \
  -Dexec.mainClass=com.mozilla.telemetry.Decoder \
  -Dexec.args=" \
    --jobName=$JOB_NAME \
    --project=$PROJECT \
    --autoscalingAlgorithm=THROUGHPUT_BASED \
    --enableStreamingEngine=false \
    --logIngestionEnabled=true \
    --errorOutput=projects/akomar-server-telemetry-poc/topics/telemetry-error \
    --errorOutputNumShards=60 \
    --errorOutputType=pubsub \
    --geoCityDatabase=gs://akomar-server-telemetry-poc/GeoIP2-City/20230616/GeoIP2-City.mmdb \
    --geoCityFilter=gs://akomar-server-telemetry-poc/cities15000.txt \
    --geoIspDatabase=gs://akomar-server-telemetry-poc/GeoIP2-ISP/20230616/GeoIP2-ISP.mmdb \
    --input=projects/akomar-server-telemetry-poc/subscriptions/glean-server-event-sub \
    --inputType=pubsub \
    --maxNumWorkers=10 \
    --numWorkers=1 \
    --output=projects/akomar-server-telemetry-poc/topics/telemetry-decoded \
    --outputFileFormat=json \
    --outputNumShards=200 \
    --outputType=pubsub \
    --proxyIps=35.227.207.240,34.120.208.123 \
    --region=us-west1 \
    --runner=DataflowRunner \
    --schemasLocation=gs://akomar-server-telemetry-poc/schemas/202306150145_ca90db03.tar.gz \
    --windowDuration=1m \
    --workerMachineType=e2-standard-2 \
"
