package main

import (
	"encoding/json"
	"fmt"
	"log"
	"path"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/kelseyhightower/envconfig"
	"github.com/masahide/discord-bot/pkg/interaction"
	"github.com/masahide/discord-bot/pkg/state"
)

type specification struct {
	Timezone  string
	SSMPath   string
	QueueURL  string
	TableName string
}

func main() {
	h := &Handler{}
	err := envconfig.Process("", &h.env)
	if err != nil {
		log.Fatal(err.Error())
	}
	sess := session.Must(session.NewSession())
	h.ssm = ssm.New(sess)
	h.ec2 = ec2.New(sess)
	h.State = state.New(sess, h.env.TableName, h.env.QueueURL)
	res, err := h.ssm.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(path.Join(h.env.SSMPath, "instanceid")),
	})
	if err != nil {
		log.Fatal(err.Error())
	}
	h.instanceID = aws.StringValue(res.Parameter.Value)
	lambda.Start(h.handler)
}

type Handler struct {
	env specification
	ssm ssmiface.SSMAPI
	ec2 ec2iface.EC2API
	*state.State
	instanceID string
}

var (
	stateMesMap = map[string]string{
		state.StateStartPending: "🖥️すでに起動指示があり、現在サーバー起動途中でした。",
		state.StateRunning:      "🖥️すでに起動していました。",
		state.StateStopPending:  "🖥️現在サーバ停止作業中でした。",
	}
)

func (h *Handler) handler(request events.APIGatewayProxyRequest) error {
	//log.Printf(dump(map[string]interface{}{"request": request}))
	data := interaction.Data{}
	if err := json.Unmarshal([]byte(request.Body), &data); err != nil {
		log.Printf("json.Unmarshal(request.Body) err:%s, request:%s", err, state.Dump(request))
		return err
	}
	if data.Data.Name == "start" {
		r, err := h.GetState(h.instanceID)
		if err != nil {
			log.Printf("GetState err:%s", err)
		}
		if time.Now().Before(time.Unix(int64(r.TTL), 0)) && r.State != state.StateStopped {
			data.Post(stateMesMap[r.State])
			if r.State != state.StateStopPending {
				h.State.SendMessage(state.Message{
					Type: state.MessageShowIP,
					Data: data,
				})
			}
			return nil
		}
		if err := h.StartState(h.instanceID); err != nil {
			if ae, ok := err.(awserr.RequestFailure); ok && ae.Code() == "ConditionalCheckFailedException" {
				data.Post(stateMesMap[state.StateStartPending])
				log.Printf("ConditionalCheckFailedException:%v", data)
				return nil
			} else {
				data.Post(fmt.Sprintf("サーバー起動エラー:[err:%s]", err))
				log.Printf("StartState err:%s", err)
			}
		}
		h.State.SendMessage(state.Message{
			Type: state.MessageStartServer,
			Data: data,
		})
		data.Post("🖥️サーバーを起動します👌")
		if err := h.startInstance(); err != nil {
			log.Printf("startInstance err:%s", err)
		}
	}
	return nil
}

func (h *Handler) startInstance() error {
	_, err := h.ec2.StartInstances(&ec2.StartInstancesInput{InstanceIds: []*string{&h.instanceID}})
	return err
}

func (h *Handler) checkInstance() (bool, error) {
	in := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{&ec2.Filter{Name: aws.String("tag:Name"), Values: []*string{&h.instanceID}}},
	}
	res, err := h.ec2.DescribeInstances(in)
	if err != nil {
		log.Printf("DescribeInstances err:%s", err)
		return false, err
	}
	for _, r := range res.Reservations {
		for _, i := range r.Instances {
			if isIgnoreInstance(i) {
				log.Printf("ignore Instance: %s, state:%s", aws.StringValue(i.InstanceId), aws.StringValue(i.State.Name))
				continue
			}
			return isRunningInstance(i), nil
		}
	}
	return false, nil
}

func isIgnoreInstance(i *ec2.Instance) bool {
	switch aws.StringValue(i.State.Name) {
	case ec2.InstanceStateNameTerminated, ec2.InstanceStateNameShuttingDown:
		return true
	}
	return false
}

func isRunningInstance(i *ec2.Instance) bool {
	switch aws.StringValue(i.State.Name) {
	case ec2.InstanceStateNameRunning, ec2.InstanceStateNameStopping, ec2.InstanceStateNamePending:
		return true
	}
	return false
}
