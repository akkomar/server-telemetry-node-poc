## Running locally
```
npm start
```


## Deploying
```
export region=us-east1
export zone=${region}-b
export project_id=akomar-server-telemetry-poc
gcloud config set compute/zone ${zone}
gcloud config set project ${project_id}
```

```
docker build --platform linux/amd64 -t test-js-logger test-js-logger && docker tag test-js-logger gcr.io/${project_id}/test-js-logger && docker push gcr.io/${project_id}/test-js-logger
```

```
gcloud container clusters create custom-fluentbit \
--zone $zone \
--logging=SYSTEM,WORKLOAD
```

```
kubectl apply -f kubernetes/test-js-logger-deploy.yaml
```

