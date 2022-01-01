package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/kelseyhightower/envconfig"
	"github.com/masahide/discord-bot/pkg/interaction"
)

type specification struct {
	Timezone string
	SSMPath  string
}

func main() {
	h := &Handler{}
	err := envconfig.Process("", &h.env)
	if err != nil {
		log.Fatal(err.Error())
	}
	lambda.Start(h.handler)
}

type Handler struct {
	env specification
}

func (h *Handler) handler(request events.APIGatewayProxyRequest) error {
	//log.Printf(dump(map[string]interface{}{"request": request}))
	var data interaction.Data
	if err := json.Unmarshal([]byte(request.Body), &data); err != nil {
		log.Printf("json.Unmarshal(request.Body) err:%s, request:%s", err, dump(request))
		return err
	}
	// handle command
	response := &WebhookInput{
		Content: "hogehogehoge fuga\n日本語",
	}
	var responsePayload bytes.Buffer
	if err := json.NewEncoder(&responsePayload).Encode(response); err != nil {
		log.Printf("responsePayload encode err:%s", err)
		return err
	}

	//log.Printf("URL:%s, res:%s", data.FollowpURL(), dump(response))
	res, err := http.Post(data.FollowpURL(), "application/json", &responsePayload)
	if err != nil {
		log.Printf("ResponseURL Post err:%s, URL:%s", err, data.FollowpURL())
		return err
	}
	defer res.Body.Close()

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Printf("ReadAll(res.Body) err:%s", err)
		return err
	}

	log.Printf(dump(map[string]interface{}{"type": "200OK", "request": data, "res": response, "post_res": string(b)}))
	return nil
}

func dump(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("json.Marshal err:%s, v:%q", err, v)
	}
	return string(b)
}

type WebhookInput struct {
	Content   string `json:"content"`
	Username  string `json:"username,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	TTS       bool   `json:"tts,omitempty"`

	// FIELD	TYPE	DESCRIPTION	REQUIRED
	// content	string	the message contents (up to 2000 characters)	one of content, file, embeds
	// username	string	override the default username of the webhook	false
	// avatar_url	string	override the default avatar of the webhook	false
	// tts	boolean	true if this is a TTS message	false
	// embeds	array of up to 10 embed objects	embedded rich content	one of content, file, embeds
	// allowed_mentions	allowed mention object	allowed mentions for the message	false
	// components *	array of message component	the components to include with the message	false
	// files[n] **	file contents	the contents of the file being sent	one of content, file, embeds
	// payload_json **	string	JSON encoded body of non-file params	multipart/form-data only
	// attachments **	array of partial attachment objects	attachment objects with filename and description	false
}
