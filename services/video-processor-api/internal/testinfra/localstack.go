//go:build integration

package testinfra

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tclocalstack "github.com/testcontainers/testcontainers-go/modules/localstack"
)

type AWS struct {
	Endpoint string
	S3       *awss3.Client
	SNS      *awssns.Client
	SQS      *awssqs.Client
}

func StartLocalStack(t *testing.T) AWS {
	t.Helper()
	ctx := context.Background()
	container, err := tclocalstack.Run(ctx, "localstack/localstack:4",
		testcontainers.WithEnv(map[string]string{"SERVICES": "s3,sns,sqs"}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	port, err := container.MappedPort(ctx, "4566/tcp")
	require.NoError(t, err)
	host, err := container.Host(ctx)
	require.NoError(t, err)
	endpoint := "http://" + host + ":" + port.Port()

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	require.NoError(t, err)
	return AWS{
		Endpoint: endpoint,
		S3: awss3.NewFromConfig(cfg, func(o *awss3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		}),
		SNS: awssns.NewFromConfig(cfg, func(o *awssns.Options) { o.BaseEndpoint = aws.String(endpoint) }),
		SQS: awssqs.NewFromConfig(cfg, func(o *awssqs.Options) { o.BaseEndpoint = aws.String(endpoint) }),
	}
}

func (a AWS) CreateBucket(t *testing.T, name string) {
	t.Helper()
	_, err := a.S3.CreateBucket(context.Background(), &awss3.CreateBucketInput{Bucket: aws.String(name)})
	require.NoError(t, err)
}

func (a AWS) CreateTopic(t *testing.T, name string) string {
	t.Helper()
	out, err := a.SNS.CreateTopic(context.Background(), &awssns.CreateTopicInput{Name: aws.String(name)})
	require.NoError(t, err)
	return aws.ToString(out.TopicArn)
}

func (a AWS) CreateQueueSubscribedTo(t *testing.T, queueName, topicARN string) string {
	t.Helper()
	ctx := context.Background()
	q, err := a.SQS.CreateQueue(ctx, &awssqs.CreateQueueInput{QueueName: aws.String(queueName)})
	require.NoError(t, err)
	attrs, err := a.SQS.GetQueueAttributes(ctx, &awssqs.GetQueueAttributesInput{
		QueueUrl: q.QueueUrl, AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
	})
	require.NoError(t, err)
	_, err = a.SNS.Subscribe(ctx, &awssns.SubscribeInput{
		TopicArn: aws.String(topicARN), Protocol: aws.String("sqs"),
		Endpoint:   aws.String(attrs.Attributes[string(sqstypes.QueueAttributeNameQueueArn)]),
		Attributes: map[string]string{"RawMessageDelivery": "true"},
	})
	require.NoError(t, err)
	return aws.ToString(q.QueueUrl)
}

func (a AWS) Receive(t *testing.T, queueURL string) []string {
	t.Helper()
	out, err := a.SQS.ReceiveMessage(context.Background(), &awssqs.ReceiveMessageInput{
		QueueUrl: aws.String(queueURL), MaxNumberOfMessages: 10, WaitTimeSeconds: 5,
	})
	require.NoError(t, err)
	var bodies []string
	for _, m := range out.Messages {
		bodies = append(bodies, aws.ToString(m.Body))
		_, err := a.SQS.DeleteMessage(context.Background(), &awssqs.DeleteMessageInput{QueueUrl: aws.String(queueURL), ReceiptHandle: m.ReceiptHandle})
		require.NoError(t, err)
	}
	return bodies
}

func (a AWS) PublishJSON(t *testing.T, topicARN string, body any) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	_, err = a.SNS.Publish(context.Background(), &awssns.PublishInput{TopicArn: aws.String(topicARN), Message: aws.String(string(raw))})
	require.NoError(t, err)
}
