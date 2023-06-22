https://cloud.google.com/community/tutorials/kubernetes-engine-customize-fluentbit



export region=us-east1
export zone=${region}-b
export project_id=akomar-server-telemetry-poc
gcloud config set compute/zone ${zone}
gcloud config set project ${project_id}


