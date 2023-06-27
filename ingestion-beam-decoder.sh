#!/bin/bash
set -eo pipefail

# This script assumes it's being run from the ingestion-beam directory of the gcp-ingestion repo
# with the code set to current contents of the main branch.

PROJECT="akomar-server-telemetry-poc"
JOB_NAME="server-events-decoder"
# : "${START_DATE:=2022-10-26}"
# : "${END_DATE:=2023-06-09}"
#   --bqRowRestriction=\"DATE(submission_timestamp) BETWEEN '$START_DATE' AND '$END_DATE'\"

set -x

#   clean \
#   compile \
mvn \
  -pl ingestion-beam \
  exec:java -Dexec.mainClass=com.mozilla.telemetry.Decoder -Dexec.args="\
      --runner=Dataflow \
      --jobName=$JOB_NAME \
      --project=$PROJECT \
      --geoCityDatabase=gs://akomar-server-telemetry-poc/GeoIP2-City/20230616/GeoIP2-City.mmdb \
      --geoIspDatabase=gs://akomar-server-telemetry-poc/GeoIP2-ISP/20230616/GeoIP2-ISP.mmdb \
      --geoCityFilter=gs://akomar-server-telemetry-poc/cities15000.txt \
      --schemasLocation=gs://akomar-server-telemetry-poc/schemas/202306150145_ca90db03.tar.gz \
      --inputType=bigquery_table \
      --input=\"$PROJECT:glean_server_event.decoder_input\" \
      --bqReadMethod=storageapi \
      --outputType=bigquery \
      --bqWriteMethod=file_loads \
      --bqClusteringFields=submission_timestamp \
      --output=$PROJECT:\${document_namespace}_live.\${document_type}_v\${document_version} \
      --errorOutputType=bigquery \
      --errorOutput=$PROJECT:payload_bytes_error.structured \
      --experiments=shuffle_mode=service \
      --region=us-east1 \
      --usePublicIps=false \
      --gcsUploadBufferSizeBytes=4194304 \
    "
