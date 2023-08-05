package com.mozilla.test;


import com.google.cloud.pubsub.v1.stub.GrpcSubscriberStub;
import com.google.cloud.pubsub.v1.stub.SubscriberStub;
import com.google.cloud.pubsub.v1.stub.SubscriberStubSettings;
import com.google.protobuf.ByteString;
import com.google.pubsub.v1.*;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.util.ArrayList;
import java.util.List;
import java.util.zip.GZIPInputStream;


public class App {
    public static void main(String... args) throws Exception {

        String projectId = "akomar-server-telemetry-poc";
        String subscriptionId = "telemetry-decoded-sub";
        // String subscriptionId = "test-preview";

        Integer numOfMessages = 10;

        subscribeSyncExample(projectId, subscriptionId, numOfMessages);
    }

    public static void subscribeSyncExample(
            String projectId, String subscriptionId, Integer numOfMessages) throws IOException {
        SubscriberStubSettings subscriberStubSettings =
                SubscriberStubSettings.newBuilder()
                        .setTransportChannelProvider(
                                SubscriberStubSettings.defaultGrpcTransportProviderBuilder()
                                        .setMaxInboundMessageSize(20 * 1024 * 1024) // 20MB (maximum message size).
                                        .build())
                        .build();

        try (SubscriberStub subscriber = GrpcSubscriberStub.create(subscriberStubSettings)) {
            String subscriptionName = ProjectSubscriptionName.format(projectId, subscriptionId);
            PullRequest pullRequest =
                    PullRequest.newBuilder()
                            .setMaxMessages(numOfMessages)
                            .setSubscription(subscriptionName)
                            .build();

            // Use pullCallable().futureCall to asynchronously perform this operation.
            PullResponse pullResponse = subscriber.pullCallable().call(pullRequest);

            // Stop the program if the pull response is empty to avoid acknowledging
            // an empty list of ack IDs.
            if (pullResponse.getReceivedMessagesList().isEmpty()) {
                System.out.println("No message was pulled. Exiting.");
                return;
            }

            List<String> ackIds = new ArrayList<>();
            for (ReceivedMessage message : pullResponse.getReceivedMessagesList()) {
                // Handle received message
                // ...
                PubsubMessage pubsubMessage = message.getMessage();
                ByteString byteStringData = pubsubMessage.getData();
                System.out.println("====================================");
                System.out.println(pubsubMessage.getAttributesMap());

                byte[] byteArray = byteStringData.toByteArray();
                final ByteArrayInputStream dataStream = new ByteArrayInputStream(byteArray);
                final ByteString decompressed;
                try {
                    final GZIPInputStream gzipStream = new GZIPInputStream(dataStream);
                    decompressed = ByteString.readFrom(gzipStream);
                    System.out.println(decompressed.toStringUtf8());
                } catch (IOException ignore) {
                }

                System.out.println("====================================");
                System.out.println();
                ackIds.add(message.getAckId());
            }

            // Acknowledge received messages.
            AcknowledgeRequest acknowledgeRequest =
                    AcknowledgeRequest.newBuilder()
                            .setSubscription(subscriptionName)
                            .addAllAckIds(ackIds)
                            .build();

            // Use acknowledgeCallable().futureCall to asynchronously perform this operation.
            subscriber.acknowledgeCallable().call(acknowledgeRequest);
//     System.out.println(pullResponse.getReceivedMessagesList());
        }
    }
}
