package etcd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"ferlab/configurations-auto-updater/model"

	"google.golang.org/grpc/codes"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdClient struct {
	Client *clientv3.Client
	Retries uint64
	RequestTimeout uint64
}

func Connect(
	userCertPath string, 
	userKeyPath string,
	Username string,
	Password string, 
	caCertPath string, 
	etcdEndpoints string, 
	connectionTimeout uint64,
	requestTimeout uint64,
	retries uint64,
	) (*EtcdClient, error) {
	tlsConf := &tls.Config{}

	//User credentials
	if Username == "" {
		certData, err := tls.LoadX509KeyPair(userCertPath, userKeyPath)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to load user credentials: %s", err.Error()))
		}
		(*tlsConf).Certificates = []tls.Certificate{certData}
	}

	(*tlsConf).InsecureSkipVerify = false
	
	//CA cert
	caCertContent, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to read root certificate file: %s", err.Error()))
	}
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(caCertContent)
	if !ok {
		return nil, errors.New("Failed to parse root certificate authority")
	}
	(*tlsConf).RootCAs = roots
	
	//Connection
	var cli *clientv3.Client
	var connErr error

	if Username == "" {
		cli, connErr = clientv3.New(clientv3.Config{
			Endpoints:   strings.Split(etcdEndpoints, ","),
			TLS:         tlsConf,
			DialTimeout: time.Duration(connectionTimeout) * time.Second,
		})
	} else {
		cli, connErr = clientv3.New(clientv3.Config{
			Username: Username,
			Password: Password,
			Endpoints:   strings.Split(etcdEndpoints, ","),
			TLS:         tlsConf,
			DialTimeout: time.Duration(connectionTimeout) * time.Second,
		})
	}
	
	if connErr != nil {
		return nil, errors.New(fmt.Sprintf("Failed to connect to etcd servers: %s", connErr.Error()))
	}
	
	return &EtcdClient{
		Client:         cli,
		Retries:        retries,
		RequestTimeout: requestTimeout,
	}, nil
}

func (cli *EtcdClient) getKeyRangeWithRetries(key string, rangeEnd string, retries uint64) (map[string]model.KeyInfo, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cli.RequestTimeout)*time.Second)
	defer cancel()

	keys := make(map[string]model.KeyInfo)

	res, err := cli.Client.Get(ctx, key, clientv3.WithRange(rangeEnd))
	if err != nil {
		etcdErr, ok := err.(rpctypes.EtcdError)
		if !ok {
			return keys, -1, err
		}
		
		if etcdErr.Code() != codes.Unavailable || retries == 0 {
			return keys, -1, err
		}

		time.Sleep(100 * time.Millisecond)
		return cli.getKeyRangeWithRetries(key, rangeEnd, retries - 1)
	}

	for _, kv := range res.Kvs {
		key, value, createRevision, modRevision, version, lease := string(kv.Key), string(kv.Value), kv.CreateRevision, kv.ModRevision, kv.Version, kv.Lease
		keys[key] = model.KeyInfo{
			Key: key,
			Value: value,
			Version: version,
			CreateRevision: createRevision,
			ModRevision: modRevision,
			Lease: lease,
		}
	}

	return keys, res.Header.Revision, nil
}

func (cli *EtcdClient) GetKeyRange(key string, rangeEnd string) (map[string]model.KeyInfo, int64, error) {
	return cli.getKeyRangeWithRetries(key, rangeEnd, cli.Retries)
}

type PrefixChangesResult struct {
	Changes model.KeysDiff
	Error   error
}

func (cli *EtcdClient) WatchPrefixChanges(prefix string, revision int64) (<-chan PrefixChangesResult) {
	outChan := make(chan PrefixChangesResult)

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		defer close(outChan)

		wc := cli.Client.Watch(ctx, prefix, clientv3.WithPrefix(), clientv3.WithRev(revision))
		if wc == nil {
			outChan <- PrefixChangesResult{Error: errors.New("Failed to watch prefix changes: Watcher could not be established")}
			return
		}
	
		for res := range wc {
			err := res.Err()
			if err != nil {
				outChan <- PrefixChangesResult{Error: errors.New(fmt.Sprintf("Failed to watch prefix changes: %s", err.Error()))}
				return
			}
	
			output := PrefixChangesResult{
				Error: nil,
				Changes: model.KeysDiff{
					Upserts: make(map[string]string),
					Deletions: []string{},
				},
			}

			for _, ev := range res.Events {
				if ev.Type == mvccpb.DELETE {
					output.Changes.Deletions = append(output.Changes.Deletions, strings.TrimPrefix(string(ev.Kv.Key), prefix))
				} else if ev.Type == mvccpb.PUT {
					output.Changes.Upserts[strings.TrimPrefix(string(ev.Kv.Key), prefix)] = string(ev.Kv.Value)
				}
			}

			outChan <- output
		}
	}()

	return outChan
}

/*func (cli *EtcdClient) getZonefilesRecursive(etcdKeyPrefix string, retries uint64) (map[string]string, int64, error) {
	var cancel context.CancelFunc
	ctx := context.Background()
	zonefiles := make(map[string]string)
	if cli.RequestTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(cli.RequestTimeout) * time.Second)
		defer cancel()
	}
	res, err := cli.Client.Get(ctx, etcdKeyPrefix, clientv3.WithPrefix())
	if err != nil {
		etcdErr, ok := err.(rpctypes.EtcdError)
		if !ok {
			return zonefiles, 0, errors.New(fmt.Sprintf("Failed to retrieve zonefiles: %s", err.Error()))
		}
		
		if etcdErr.Code() != codes.Unavailable || retries == 0 {
			return zonefiles, 0, errors.New(fmt.Sprintf("Failed to retrieve zonefiles: %s", etcdErr.Error()))
		}

		time.Sleep(time.Duration(100) * time.Millisecond)
		return cli.getZonefilesRecursive(etcdKeyPrefix, retries - 1)
	}

	for _, kv := range res.Kvs {
		zonefiles[strings.TrimPrefix(string(kv.Key), etcdKeyPrefix)] = string(kv.Value)
	}
	
	return zonefiles, res.Header.Revision, nil
}

func (cli *EtcdClient) GetZonefiles(etcdKeyPrefix string) (map[string]string, int64, error) {
	return cli.getZonefilesRecursive(etcdKeyPrefix, cli.Retries)
}

type ZonefileEvent struct {
	Domain  string
	Content string
	Action  string
	Err     error
}

func (cli *EtcdClient) WatchZonefiles(etcdKeyPrefix string, revision int64, events chan ZonefileEvent) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer close(events)
	wc := cli.Client.Watch(ctx, etcdKeyPrefix, clientv3.WithPrefix(), clientv3.WithRev(revision))
	if wc == nil {
		events <- ZonefileEvent{Err: errors.New("Failed to watch zonefiles changes: Watcher could not be established")}
		return
	}

	for res := range wc {
		err := res.Err()
		if err != nil {
			events <- ZonefileEvent{Err: errors.New(fmt.Sprintf("Failed to watch zonefiles changes: %s", err.Error()))}
			return
		}

		for _, ev := range res.Events {
			if ev.Type == mvccpb.DELETE {
				events <- ZonefileEvent{Action: "delete", Domain: strings.TrimPrefix(string(ev.Kv.Key), etcdKeyPrefix)}
			} else if ev.Type == mvccpb.PUT {
				events <- ZonefileEvent{Action: "upsert", Domain: strings.TrimPrefix(string(ev.Kv.Key), etcdKeyPrefix), Content: string(ev.Kv.Value)}
			}
		}
	}
}*/