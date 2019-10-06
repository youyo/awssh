package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2instanceconnect"
	"github.com/aws/aws-sdk-go/service/ssm"
	homedir "github.com/mitchellh/go-homedir"
)

const (
	DocumentNameAwsStartPortForwardingSession string = "AWS-StartPortForwardingSession"
)

type (
	SsmDocument struct {
		Target       string                `json:"Target"`
		DocumentName string                `json:"DocumentName"`
		Parameters   SsmDocumentParameters `json:"Parameters"`
	}
	SsmDocumentParameters struct {
		PortNumber      []string `json:"portNumber"`
		LocalPortNumber []string `json:"localPortNumber"`
	}

	Instance struct {
		ID      string
		TagName string
	}
	Instances []Instance
)

func NewAwsSession() (sess *session.Session) {
	sess = session.Must(
		session.NewSessionWithOptions(
			session.Options{
				SharedConfigState: session.SharedConfigEnable,
			},
		),
	)
	return sess
}

func GetRegion(sess *session.Session) (region string) {
	region = *sess.Config.Region
	return region
}

func GetSsmApiUrl(region string) (url string) {
	url = "https://ssm." + region + ".amazonaws.com"
	return url
}

func GetSsmSessionToken(ctx context.Context, sess *session.Session, instanceID, remotePortNumber, localPortNumber string) (tokens, sessionManagerParam string, err error) {
	ssmClient := ssm.New(sess)
	ssmInput := &ssm.StartSessionInput{
		Target:       aws.String(instanceID),
		DocumentName: aws.String(DocumentNameAwsStartPortForwardingSession),
		Parameters: map[string][]*string{
			"portNumber":      []*string{aws.String(remotePortNumber)},
			"localPortNumber": []*string{aws.String(localPortNumber)},
		},
	}
	result, err := ssmClient.StartSessionWithContext(ctx, ssmInput)
	if err != nil {
		return "", "", err
	}

	tokensBytes, err := json.Marshal(result)
	if err != nil {
		return "", "", err
	}
	tokens = string(tokensBytes)

	sessionManagerParams := SsmDocument{
		Target:       instanceID,
		DocumentName: DocumentNameAwsStartPortForwardingSession,
		Parameters: SsmDocumentParameters{
			PortNumber:      []string{remotePortNumber},
			LocalPortNumber: []string{localPortNumber},
		},
	}
	sessionManagerParamsByte, err := json.Marshal(sessionManagerParams)
	if err != nil {
		return "", "", err
	}

	sessionManagerParam = string(sessionManagerParamsByte)

	return tokens, sessionManagerParam, nil
}

func SendSSHPublicKey(ctx context.Context, sess *session.Session, instanceID, username, publickeyFilePath string) (err error) {
	az, err := GetInstanceAZ(ctx, sess, instanceID)
	if err != nil {
		return err
	}

	publicKey, err := ReadPublicKey(publickeyFilePath)
	if err != nil {
		return err
	}

	ec2InstanceConnectClient := ec2instanceconnect.New(sess)
	ec2InstanceConnectInput := &ec2instanceconnect.SendSSHPublicKeyInput{
		AvailabilityZone: aws.String(az),
		InstanceId:       aws.String(instanceID),
		InstanceOSUser:   aws.String(username),
		SSHPublicKey:     aws.String(publicKey),
	}
	if result, err := ec2InstanceConnectClient.SendSSHPublicKeyWithContext(ctx, ec2InstanceConnectInput); err != nil {
		return err
	} else if !*result.Success {
		return errors.New("SendSSHPublicKey request unsuccessful.")
	}

	return nil
}

func ReadPublicKey(filePath string) (publicKey string, err error) {
	fullPath, err := homedir.Expand(filePath)
	if err != nil {
		return "", err
	}

	publicKeyBytes, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	publicKey = string(publicKeyBytes)
	return publicKey, nil
}

func GetInstanceAZ(ctx context.Context, sess *session.Session, instanceID string) (az string, err error) {
	ec2Client := ec2.New(sess)
	instance, err := GetInstance(ctx, sess, instanceID)
	if err != nil {
		return "", err
	}

	subnetId := instance.SubnetId
	subnetInput := &ec2.DescribeSubnetsInput{
		SubnetIds: []*string{subnetId},
	}
	subnetResult, err := ec2Client.DescribeSubnetsWithContext(ctx, subnetInput)
	if err != nil {
		return "", err
	}

	for _, subnet := range subnetResult.Subnets {
		az = *subnet.AvailabilityZone
	}

	return az, nil
}

func GetInstance(ctx context.Context, sess *session.Session, instanceID string) (instance *ec2.Instance, err error) {
	ec2Client := ec2.New(sess)
	ec2Input := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			aws.String(instanceID),
		},
	}
	result, err := ec2Client.DescribeInstancesWithContext(ctx, ec2Input)
	if err != nil {
		return nil, err
	}

	for _, reservation := range result.Reservations {
		instance = reservation.Instances[0]
	}
	return instance, nil
}

func GetRunningInstances(ctx context.Context, sess *session.Session) (instances Instances, err error) {
	ec2Client := ec2.New(sess)
	ec2Input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("instance-state-name"),
				Values: []*string{
					aws.String("running"),
				},
			},
		},
	}
	result, err := ec2Client.DescribeInstancesWithContext(ctx, ec2Input)
	if err != nil {
		return nil, err
	}

	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			i := Instance{
				ID: *instance.InstanceId,
			}
			for _, tag := range instance.Tags {
				if *tag.Key == "Name" {
					i.TagName = *tag.Value
				}
			}
			instances = append(instances, i)
		}
	}
	return instances, nil
}
