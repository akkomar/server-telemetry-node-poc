package com.mozilla.test;


import com.google.cloud.pubsub.v1.stub.GrpcSubscriberStub;
import com.google.cloud.pubsub.v1.stub.SubscriberStub;
import com.google.cloud.pubsub.v1.stub.SubscriberStubSettings;
import com.google.pubsub.v1.AcknowledgeRequest;
import com.google.pubsub.v1.ProjectSubscriptionName;
import com.google.pubsub.v1.PullRequest;
import com.google.pubsub.v1.PullResponse;
import com.google.pubsub.v1.ReceivedMessage;
import java.io.IOException;
import java.util.ArrayList;
import java.util.List;


public class App 
{
    public static void main(String... args) throws Exception {

    String projectId = "akomar-server-telemetry-poc";
    String subscriptionId = "test-preview";
    Integer numOfMessages = 1;

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
      System.out.println(pullResponse.getReceivedMessagesList());
    }
  }
}
