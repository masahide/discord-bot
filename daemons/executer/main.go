package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/k0kubun/pp"
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
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	h := &Handler{}
	err := envconfig.Process("", &h.env)
	if err != nil {
		log.Fatal(err.Error())
	}
	sess := session.Must(session.NewSession())
	h.ssm = ssm.New(sess)
	h.State = state.New(sess, h.env.TableName, h.env.QueueURL)
	key := path.Join(h.env.SSMPath, "instanceid")
	res, err := h.ssm.GetParameter(&ssm.GetParameterInput{Name: aws.String(key)})
	if err != nil {
		log.Fatal(err.Error())
	}
	h.instanceID = aws.StringValue(res.Parameter.Value)

	if len(h.instanceID) == 0 {
		log.Fatalf("instanceID cannot be obtained from parameterStore:%s", key)
	}
	/*
		r, err := h.GetState(h.instanceID)
		if err != nil {
			log.Fatalf("table:%s, errType:%T err:%s", h.env.TableName, err, err)
		}
		log.Println("record:", r)
		if err := h.StartState(h.instanceID); err != nil {
			if ae, ok := err.(awserr.RequestFailure); ok && ae.Code() == "ConditionalCheckFailedException" {
				log.Printf("すでにスタート済み")
			} else {
				log.Printf("StartState err:%s", err)
			}
		}
	*/

	/*
		if err := h.PutState(h.instanceID, state.StateRunning); err != nil {
			log.Fatalf("PutState err:%s", err)
		}
	*/

	h.receiveMes()
}

type Handler struct {
	env specification
	ssm ssmiface.SSMAPI
	*state.State
	instanceID string
}

func (h *Handler) receiveMes() {
	postTTL := time.Now().Add(-1 * time.Second)
	postfunc := func() {
		if time.Now().After(postTTL) {
			if err := h.PutState(h.instanceID, state.StateRunning); err != nil {
				log.Printf("PutState err:%s", err)
			}
			postTTL = time.Now().Add(5 * time.Minute)
			//log.Println("PutState.")
			return
		}
		//log.Println("ok")
	}
	for {
		postfunc()
		r, err := h.ReceiveMessage()
		if err != nil {
			log.Printf("ReceiveMessage err:%s", err)
			time.Sleep(5 * time.Second)
			continue
		}
		for _, mes := range r.Messages {
			m := state.Message{}
			err := json.Unmarshal([]byte(aws.StringValue(mes.Body)), &m)
			if err != nil {
				log.Printf("ReceiveMessage json.Unmarshal err:%s, body:[%s]", err, aws.StringValue(mes.Body))
				if err := h.DeleteMessage(mes); err != nil {
					log.Printf("DeleteMessage err:%s", err)
				}
				continue
			}
			h.execute(m, mes)
		}
	}
}

func (h *Handler) execute(m state.Message, org *sqs.Message) {
	switch m.Type {
	case state.MessageStartServer:
		ip := getPublicIP()
		m.Data.Post(fmt.Sprintf("サーバーのIPは `%s` になりました", ip))
		if err := h.DeleteMessage(org); err != nil {
			log.Printf("DeleteMessage err:%s", err)
		}
	case state.MessageShowIP:
		ip := getPublicIP()
		m.Data.Post(fmt.Sprintf("サーバーIP: `%s`", ip))
		if err := h.DeleteMessage(org); err != nil {
			log.Printf("DeleteMessage err:%s", err)
		}
	default:
		log.Println("execute default action...")
		pp.Println(m)
		if err := h.DeleteMessage(org); err != nil {
			log.Printf("DeleteMessage err:%s", err)
		}
	}

}

func getPublicIP() string {
	resp, err := http.Get("http://checkip.amazonaws.com")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return string(body)
}

func (h *Handler) handler(data interaction.Data) error {
	//log.Printf(dump(map[string]interface{}{"request": request}))
	/*
		var data interaction.Data
		if err := json.Unmarshal([]byte(request.Body), &data); err != nil {
			log.Printf("json.Unmarshal(request.Body) err:%s, request:%s", err, dump(request))
			return err
		}
	*/
	// handle command
	return nil
}
