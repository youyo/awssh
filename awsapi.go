package awssh

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2instanceconnect"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/manifoldco/promptui"
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

func newAwsSession(profile string, cache bool, duration time.Duration) (sess *session.Session) {
	if cache {
		c, _ := NewCache(CachePath, profile)
		credsCache, err := c.Load()
		if err != nil {
			sess = session.Must(
				session.NewSessionWithOptions(
					session.Options{
						SharedConfigState:       session.SharedConfigEnable,
						Profile:                 profile,
						AssumeRoleDuration:      duration,
						AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
					},
				),
			)
			credsCache, _ := sess.Config.Credentials.Get()
			c.Save(&credsCache, duration)
		} else {
			creds := credentials.NewStaticCredentialsFromCreds(*credsCache)
			sess = session.Must(
				session.NewSessionWithOptions(
					session.Options{
						Config: aws.Config{
							Credentials: creds,
						},
						SharedConfigState:       session.SharedConfigEnable,
						Profile:                 profile,
						AssumeRoleDuration:      duration,
						AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
					},
				),
			)
		}
	} else {
		sess = session.Must(
			session.NewSessionWithOptions(
				session.Options{
					SharedConfigState:       session.SharedConfigEnable,
					Profile:                 profile,
					AssumeRoleDuration:      duration,
					AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
				},
			),
		)
	}

	return sess
}

func getRegion(sess *session.Session) (region string) {
	region = *sess.Config.Region
	return region
}

func getSsmApiUrl(region string) (url string) {
	url = "https://ssm." + region + ".amazonaws.com"
	return url
}

func getSsmSessionToken(ctx context.Context, sess *session.Session, instanceID, remotePortNumber, localPortNumber string) (tokens, sessionManagerParam string, err error) {
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

func sendSSHPublicKey(ctx context.Context, sess *session.Session, instanceID, username, publickeyFilePath string) (err error) {
	az, err := getInstanceAZ(ctx, sess, instanceID)
	if err != nil {
		return err
	}

	publicKey, err := readPublicKey(publickeyFilePath)
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

func getInstanceAZ(ctx context.Context, sess *session.Session, instanceID string) (az string, err error) {
	ec2Client := ec2.New(sess)
	instance, err := getInstance(ctx, sess, instanceID)
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

func getInstance(ctx context.Context, sess *session.Session, instanceID string) (instance *ec2.Instance, err error) {
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

func getRunningInstances(ctx context.Context, sess *session.Session) (instances Instances, err error) {
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
	if len(instances) == 0 {
		err = errors.New("No running instance")
		return nil, err
	}

	return instances, nil
}

func selectInstance(instances Instances) (instanceID string, err error) {
	prompt := promptui.Select{
		Label: "Instances",
		Templates: &promptui.SelectTemplates{
			Label:    `{{ . | green }}`,
			Active:   `{{ ">" | blue }} {{ .ID | red }} {{ .TagName | red }}`,
			Inactive: `{{ .ID | cyan }} {{ .TagName | cyan }}`,
			Selected: `{{ .ID | yellow }} {{ .TagName | yellow }}`,
		},
		Items: instances,
		Size:  50,
		Searcher: func(input string, index int) bool {
			item := instances[index]
			instanceName := strings.Replace(strings.ToLower(item.TagName), " ", "", -1)
			instanceID := strings.Replace(strings.ToLower(item.ID), " ", "", -1)
			input = strings.Replace(strings.ToLower(input), " ", "", -1)
			if strings.Contains(instanceName, input) {
				return true
			} else if strings.Contains(instanceID, input) {
				return true
			}
			return false
		},
		StartInSearchMode: true,
	}

	index, _, err := prompt.Run()
	if err != nil {
		return "", err
	}

	instanceID = instances[index].ID

	return instanceID, nil
}

func createAMI(ctx context.Context, sess *session.Session, instanceID string) (imageId *string, err error) {
	t := time.Now()
	now := t.Format("20060102150405")

	ec2Client := ec2.New(sess)
	ec2Input := &ec2.CreateImageInput{
		Description: aws.String("Created by awssh command to auto snapshot. [" + instanceID + "]"),
		InstanceId:  aws.String(instanceID),
		Name:        aws.String(instanceID + "_" + now),
		NoReboot:    aws.Bool(true),
	}
	result, err := ec2Client.CreateImageWithContext(ctx, ec2Input)
	if err != nil {
		return nil, err
	}

	imageId = result.ImageId

	input := &ec2.CreateTagsInput{
		Resources: []*string{
			imageId,
		},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("instance-id"),
				Value: aws.String(instanceID),
			},
			{
				Key:   aws.String("Created"),
				Value: aws.String("awssh"),
			},
		},
	}

	if _, err := ec2Client.CreateTags(input); err != nil {
		return nil, err
	}

	return imageId, nil
}
