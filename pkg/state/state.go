package state

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"github.com/masahide/discord-bot/pkg/interaction"
)

type Message struct {
	Data interaction.Data
	Type string
}

type Record struct {
	ID    string `dynamodbav:"id"`
	State string `dynamodbav:"state,omitempty"`
	TTL   int    `dynamodbav:"ttl,unixtime,,omitempty"`
}

const (
	StateStopped      = "stopped"      // 停止中
	StateStartPending = "startPending" // 開始中
	StateRunning      = "running"      // 実行中
	StateStopPending  = "stopPending"  // 停止処理中

	MessageStartServer = "startServer" // startServer
	MessageStopServer  = "stopServer"
	MessageShowIP      = "showIP" // show ip
)

type State struct {
	dyndb     dynamodbiface.DynamoDBAPI
	sqs       sqsiface.SQSAPI
	tableName string
	queueURL  string
}

func New(sess *session.Session, tableName, queueURL string) *State {
	return &State{
		dyndb:     dynamodb.New(sess),
		sqs:       sqs.New(sess),
		tableName: tableName,
		queueURL:  queueURL,
	}
}

func (s *State) StartState(id string) error {
	update := expression.
		Set(expression.Name("state"), expression.Value(StateStartPending)).
		Set(expression.Name("ttl"), expression.Value(time.Now().Add(5*time.Minute).Unix())).
		Set(expression.Name("UpdatedAt"), expression.Value(time.Now()))
	cond := expression.Name("state").Equal(expression.Value(StateStopped)).
		Or(expression.Name("ttl").LessThanEqual(expression.Value(time.Now().Unix())).
			Or(expression.AttributeNotExists(expression.Name("id"))))
	expr, err := expression.NewBuilder().
		WithUpdate(update).
		WithCondition(cond).
		Build()
	if err != nil {
		return err
	}
	av, err := dynamodbattribute.MarshalMap(Record{ID: id})
	if err != nil {
		panic(fmt.Sprintf("failed to DynamoDB marshal Record, %v", err))
	}
	_, err = s.dyndb.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: &s.tableName,
		Key:       av,

		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
		ConditionExpression:       expr.Condition(),
	})
	return err
}

func (s *State) PutState(id, newState string) error {
	r := Record{
		ID:    id,
		State: newState,
		TTL:   int(time.Now().Add(4 * time.Minute).Unix()),
	}
	av, err := dynamodbattribute.MarshalMap(r)
	if err != nil {
		panic(fmt.Sprintf("failed to DynamoDB marshal Record, %v", err))
	}

	_, err = s.dyndb.PutItem(&dynamodb.PutItemInput{
		TableName: &s.tableName,
		Item:      av,
	})
	return err
}

func (s *State) GetState(id string) (Record, error) {
	k, err := dynamodbattribute.MarshalMap(Record{ID: id})
	if err != nil {
		return Record{}, err
	}
	input := &dynamodb.GetItemInput{
		TableName: &s.tableName,
		Key:       k,
	}
	res, err := s.dyndb.GetItem(input)
	if err != nil {
		return Record{}, err
	}
	var r Record
	err = dynamodbattribute.UnmarshalMap(res.Item, &r)
	return r, err
}

func (s *State) ReceiveMessage() (*sqs.ReceiveMessageOutput, error) {
	return s.sqs.ReceiveMessage(
		&sqs.ReceiveMessageInput{
			MaxNumberOfMessages: aws.Int64(1),
			QueueUrl:            &s.queueURL,
			VisibilityTimeout:   aws.Int64(30),
			WaitTimeSeconds:     aws.Int64(20),
		},
	)
}

func (s *State) SendMessage(m Message) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = s.sqs.SendMessage(&sqs.SendMessageInput{
		QueueUrl:    &s.queueURL,
		MessageBody: aws.String(string(b)),
	})
	return err
}

func (s *State) DeleteMessage(m *sqs.Message) error {
	_, err := s.sqs.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      &s.queueURL,
		ReceiptHandle: m.ReceiptHandle,
	})
	return err
}

func Dump(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("json.Marshal err:%s, v:%q", err, v)
	}
	return string(b)
}
