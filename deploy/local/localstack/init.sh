#!/bin/sh
set -eu

awslocal s3 mb s3://video-processor
awslocal s3api put-bucket-cors --bucket video-processor --cors-configuration '{
  "CORSRules": [{
    "AllowedOrigins": ["*"],
    "AllowedMethods": ["PUT", "GET", "HEAD"],
    "AllowedHeaders": ["*"],
    "ExposeHeaders": ["ETag"],
    "MaxAgeSeconds": 3000
  }]
}'
awslocal s3api put-bucket-lifecycle-configuration --bucket video-processor --lifecycle-configuration '{
  "Rules": [{"ID": "expire-uploads", "Filter": {"Prefix": "uploads/"}, "Status": "Enabled", "Expiration": {"Days": 7}}]
}'

API_TOPIC=$(awslocal sns create-topic --name video-processor-api-events --query TopicArn --output text)
WORKER_TOPIC=$(awslocal sns create-topic --name video-processor-worker-events --query TopicArn --output text)

WORKER_DLQ=$(awslocal sqs create-queue --queue-name video-processor-worker-inbox-dlq --query QueueUrl --output text)
WORKER_DLQ_ARN=$(awslocal sqs get-queue-attributes --queue-url "$WORKER_DLQ" --attribute-names QueueArn --query Attributes.QueueArn --output text)
WORKER_INBOX=$(awslocal sqs create-queue --queue-name video-processor-worker-inbox --attributes "{\"VisibilityTimeout\":\"300\",\"RedrivePolicy\":\"{\\\"deadLetterTargetArn\\\":\\\"$WORKER_DLQ_ARN\\\",\\\"maxReceiveCount\\\":\\\"3\\\"}\"}" --query QueueUrl --output text)
WORKER_INBOX_ARN=$(awslocal sqs get-queue-attributes --queue-url "$WORKER_INBOX" --attribute-names QueueArn --query Attributes.QueueArn --output text)

API_DLQ=$(awslocal sqs create-queue --queue-name video-processor-api-inbox-dlq --query QueueUrl --output text)
API_DLQ_ARN=$(awslocal sqs get-queue-attributes --queue-url "$API_DLQ" --attribute-names QueueArn --query Attributes.QueueArn --output text)
API_INBOX=$(awslocal sqs create-queue --queue-name video-processor-api-inbox --attributes "{\"VisibilityTimeout\":\"30\",\"RedrivePolicy\":\"{\\\"deadLetterTargetArn\\\":\\\"$API_DLQ_ARN\\\",\\\"maxReceiveCount\\\":\\\"3\\\"}\"}" --query QueueUrl --output text)
API_INBOX_ARN=$(awslocal sqs get-queue-attributes --queue-url "$API_INBOX" --attribute-names QueueArn --query Attributes.QueueArn --output text)

awslocal sns subscribe --topic-arn "$API_TOPIC" --protocol sqs --notification-endpoint "$WORKER_INBOX_ARN" --attributes '{"RawMessageDelivery":"true"}'
awslocal sns subscribe --topic-arn "$WORKER_TOPIC" --protocol sqs --notification-endpoint "$API_INBOX_ARN" --attributes '{"RawMessageDelivery":"true"}'

echo "localstack ready: $API_TOPIC $WORKER_TOPIC $WORKER_INBOX $API_INBOX"
