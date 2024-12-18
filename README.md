# Glean server-side event collection prototype
This is a proof of concept of the end to end server-side telemetry collection.

Data flow:
* test-js-logger - node application, deployed to k8s, logs events to Cloud Logging
* cloud logging sink delivers events to Pub/Sub topic
* Decoder job reads from the topic ([see this PR](https://github.com/mozilla/gcp-ingestion/pull/2400)) and decodes messages to a format compatible with existing parts of the ingestion pipeline

## Development
### Running logger locally
```
cd test-js-logger
nvm use 18.14.2
npm start
```
#### Go logger
```
cd test-go-logger
go run main.go
```
#### Python logger
```
cd test-py-logger
python main.py
```

### Deploying
```
export region=us-east1
export zone=${region}-b
export project_id=akomar-server-telemetry-poc
gcloud config set compute/zone ${zone}
gcloud config set project ${project_id}
```

```
docker build --platform linux/amd64 -t test-js-logger test-js-logger && docker tag test-js-logger gcr.io/${project_id}/test-js-logger && docker push gcr.io/${project_id}/test-js-logger

docker build --platform linux/amd64 -t test-ts-logger test-ts-logger && docker tag test-ts-logger gcr.io/${project_id}/test-ts-logger && docker push gcr.io/${project_id}/test-ts-logger
```

```
gcloud container clusters create custom-fluentbit \
--zone $zone \
--logging=SYSTEM,WORKLOAD
```

```
kubectl apply -f kubernetes/test-js-logger-deploy.yaml

kubectl apply -f kubernetes/test-ts-logger-deploy.yaml
```
#### Rust logger
```
docker build --platform linux/amd64 -t test-rust-logger test-rust-logger && docker tag test-rust-logger gcr.io/${project_id}/test-rust-logger && docker push gcr.io/${project_id}/test-rust-logger

kubectl apply -f kubernetes/test-rust-logger-deploy.yaml

kubectl rollout restart deploy test-rust-logger
```
#### Go logger
```
docker build --platform linux/amd64 -t test-go-logger test-go-logger && docker tag test-go-logger gcr.io/${project_id}/test-go-logger && docker push gcr.io/${project_id}/test-go-logger

kubectl apply -f kubernetes/test-go-logger-deploy.yaml

kubectl rollout restart deploy test-go-logger
```
#### Python logger
```
docker build --platform linux/amd64 -t test-py-logger test-py-logger && docker tag test-py-logger gcr.io/${project_id}/test-py-logger && docker push gcr.io/${project_id}/test-py-logger

kubectl apply -f kubernetes/test-py-logger-deploy.yaml

kubectl rollout restart deploy test-py-logger
```

### Creating logging sink to Pub/Sub
```
gcloud logging sinks create glean-event-pubsub-sink pubsub.googleapis.com/projects/akomar-server-telemetry-poc/topics/glean-server-event --log-filter='jsonPayload.Type="glean-server-event"'
```
Run a streaming Decoder job:
```sh
cd ../gcp-ingestion
../server-telemetry-node-poc/ingestion-beam-decoder-streaming.sh
```
Run java-consumer to read a decoded message from Pub/Sub topic:
```
cd java-consumer
mvn clean compile exec:java -Dexec.mainClass=com.mozilla.test.App
```

## References
https://cloud.google.com/community/tutorials/kubernetes-engine-customize-fluentbit
