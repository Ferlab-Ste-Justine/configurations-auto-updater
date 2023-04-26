package main

import (
	"context"
	"regexp"
	"strings"

	"github.com/Ferlab-Ste-Justine/configurations-auto-updater/configs"

	"github.com/Ferlab-Ste-Justine/etcd-sdk/client"
	"github.com/Ferlab-Ste-Justine/etcd-sdk/keypb"
	"google.golang.org/grpc"
)

func GetKeyFilter(regex *regexp.Regexp) client.KeyDiffFilter {
	if regex != nil {
		return func(key string) bool {
			return regex.MatchString(key)
		}
	}

	return func(key string) bool {return true}
}

func GetKeyTransform(TrimKeyPath bool) client.KeyDiffTransform {
	if TrimKeyPath {
		return func(key string) string {
			split := strings.Split(key, "/")
			return split[len(split) - 1]
		}
	}

	return func(key string) string {return key}
}

type GrpcNotifClientTarget struct {
	conn         *grpc.ClientConn
	client       keypb.KeyPushServiceClient
	KeyFilter    client.KeyDiffFilter
	KeyTransform client.KeyDiffTransform
	MaxChunkSize uint64
}

type GrpcNotifClient struct {
	Targets []GrpcNotifClientTarget
}

func ConnectToNotifEndpoints(notifications []configs.ConfigsGrpcNotifications) (*GrpcNotifClient, error) {
	cli := GrpcNotifClient{Targets: []GrpcNotifClientTarget{}}
	for _, notification := range notifications {
		var opts []grpc.DialOption
		opts = append(opts, grpc.WithInsecure())
		
		conn, connErr := grpc.Dial(notification.Endpoint, opts...)
		if connErr != nil {
			cli.Close()
			return nil, connErr
		}

		cli.Targets = append(cli.Targets, GrpcNotifClientTarget{
			conn: conn,
			client: keypb.NewKeyPushServiceClient(conn),
			KeyFilter: GetKeyFilter(notification.FilterRegex),
			KeyTransform: GetKeyTransform(notification.TrimKeyPath),
			MaxChunkSize: notification.MaxChunkSize,
		})
	}

	return &cli, nil
}

func (cli *GrpcNotifClient) sendTo(idx int, diff *client.KeyDiff) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	target := cli.Targets[idx]

	diff = diff.FilterKeys(target.KeyFilter).TransformKeys(target.KeyTransform)

	if diff.IsEmpty() {
		return nil
	}

	stream, err := target.client.SendKeyDiff(ctx)
	if err != nil {
		return err
	}

	doneCh := make(chan struct{})
	defer close(doneCh)

	sendCh := keypb.GenSendKeyDiffRequests(*diff, target.MaxChunkSize, doneCh)
	for req := range sendCh {
		err = stream.Send(req)
		if err != nil {
			return err
		}
	}

	_, closeErr := stream.CloseAndRecv()
	if closeErr != nil {
		return closeErr
	}
	
	return nil
}

func (cli *GrpcNotifClient) Send(diff client.KeyDiff) error {
	for idx, _ := range cli.Targets {
		err := cli.sendTo(idx, &diff)
		if err != nil {
			return err
		}
	}
	return nil
}

func (cli *GrpcNotifClient) Close() []error {
	errors := []error{}
	for _, target := range cli.Targets {
		err := target.conn.Close()
		if err != nil {
			errors = append(errors ,err)
		}
	}
	return errors
}