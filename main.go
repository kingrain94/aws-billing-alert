package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// environment variable name
const (
	AccountID string = "ACCOUNT_ID"
	Webhook   string = "WEBHOOK"
)

// constants
const (
	BucketName     string = "kingraint94-billing-reports"
	YYMMTimeFormat string = "2006-01"

	// csv format
	ServiceColumnID  int = 13
	TotalColumnID    int = 28
	CurrencyColumnID int = 23
)

// Env is struct hold environment variable
var Env struct {
	AccountID string
	Webhook   string
}

// Event present for event
type Event struct {
	Name string `json:"name"`
}

func init() {
	Env.AccountID = os.Getenv(AccountID)
	Env.Webhook = os.Getenv(Webhook)
}

// HandleRequest handle request received by lambda
func HandleRequest(ctx context.Context, name Event) (string, error) {
	// get cost report from s3
	mySession := session.Must(session.NewSession())
	svc := s3.New(mySession, aws.NewConfig().WithRegion("us-east-2"))

	s3Key := fmt.Sprintf("%s-aws-billing-csv-%s.csv", Env.AccountID, time.Now().Format(YYMMTimeFormat))
	params := &s3.GetObjectInput{
		Bucket: aws.String(BucketName),
		Key:    aws.String(s3Key),
	}
	log.Println(fmt.Sprintf("Start get billing info from s3 file %s", s3Key))
	resp, err := svc.GetObject(params)
	if err != nil {
		log.Fatal(err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	reader := csv.NewReader(bytes.NewBuffer(body))
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatal(err)
	}

	// send billing info by webhook
	textMsg := fmt.Sprintf("%s Billing Report", time.Now().Format(YYMMTimeFormat))
	for k, v := range records {
		if len(v[ServiceColumnID]) == 0 {
			textMsg = fmt.Sprintf("%s\nSum:\t%s\t%s", textMsg, v[TotalColumnID], v[CurrencyColumnID])
			break
		}
		c, err := strconv.ParseFloat(v[TotalColumnID], 64)
		if err != nil && k != 0 {
			log.Fatal(err)
		}
		if c > 0 {
			textMsg = fmt.Sprintf("%s\n%s\t%s\t%s", textMsg, v[ServiceColumnID], v[TotalColumnID], v[CurrencyColumnID])
		}
	}
	var data = []byte(fmt.Sprintf(`{"type": "mrkdwn", "text":"%+v"}`, textMsg))
	request, err := http.NewRequest("POST", Env.Webhook, bytes.NewBuffer(data))
	request.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{}
	httpResponse, err := httpClient.Do(request)
	if err != nil {
		log.Fatal(err)
	}
	defer httpResponse.Body.Close()

	body, err = ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(body)

	return name.Name, nil
}

func main() {
	lambda.Start(HandleRequest)
}
