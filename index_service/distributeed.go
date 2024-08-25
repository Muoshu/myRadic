package index_service

import (
	"context"
	"fmt"
	"github.com/Muoshu/myRadic/types"
	"github.com/Muoshu/myRadic/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"sync"
	"sync/atomic"
	"time"
)

type Sentinel struct {
	// 从Hub上获取IndexServiceWorker集合。可能是直接访问ServiceHub，也可能是走代理
	hub      IServiceHub
	connPool sync.Map
}

func NewSentinel(etcdServers []string) *Sentinel {
	return &Sentinel{
		hub:      GetServiceHubProxy(etcdServers, 10, 100), //走代理HubProxy
		connPool: sync.Map{},
	}
}

func (sentinel *Sentinel) GetGrpcConn(endpoint string) *grpc.ClientConn {
	if v, ok := sentinel.connPool.Load(endpoint); ok {
		conn := v.(*grpc.ClientConn)
		//如果状态不可用，则从连接缓存中删除
		if conn.GetState() == connectivity.TransientFailure || conn.GetState() == connectivity.Shutdown {
			util.Log.Printf("connection status to endpoint %s is %s", endpoint, conn.GetState())
			conn.Close()
			sentinel.connPool.Delete(endpoint)
		} else {
			return conn
		}
	}

	//连接到服务端
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	conn, err := grpc.DialContext(
		ctx,
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		util.Log.Printf("dial %s failed: %s", endpoint, err)
		return nil
	}
	util.Log.Printf("connect to grpc server %s", endpoint)
	sentinel.connPool.Store(endpoint, conn)
	return conn
}

// 向集群中添加文档
func (sentinel *Sentinel) AddDoc(doc types.Document) (int, error) {
	// 根据负载均衡策略，选择一台index worker，把doc添加到它上面去
	endpoint := sentinel.hub.GetServiceEndpoint(INDEX_SERVICE)
	if len(endpoint) == 0 {
		return 0, fmt.Errorf("there is no alive index worker")
	}
	conn := sentinel.GetGrpcConn(endpoint)
	if conn == nil {
		return 0, fmt.Errorf("connect to worker %s failed", endpoint)
	}
	client := NewIndexServiceClient(conn)
	affected, err := client.AddDoc(context.Background(), &doc)
	if err != nil {
		return 0, err
	}
	util.Log.Printf("add %d doc to worker %s", affected.Count, endpoint)
	return int(affected.Count), nil
}

// 从集群上删除docId，返回成功删除的doc数（正常情况下不会超过1）
func (sentinel *Sentinel) DeleteDoc(docId string) int {
	endpoints := sentinel.hub.GetServiceEndpoints(INDEX_SERVICE)
	if len(endpoints) == 0 {
		return 0
	}
	var n int32
	wg := sync.WaitGroup{}
	wg.Add(len(endpoints))
	for _, endpoint := range endpoints {
		//并行到各个IndexServiceWorker上把docId删除。正常情况下只有一个worker上有该doc
		go func(endpoint string) {
			defer wg.Done()
			conn := sentinel.GetGrpcConn(endpoint)
			if conn != nil {
				client := NewIndexServiceClient(conn)
				affected, err := client.DeleteDoc(context.Background(), &DocId{docId})
				if err != nil {
					util.Log.Printf("delete doc %s from worker %s failed: %s", docId, endpoint, err)
				} else {
					if affected.Count > 0 {
						atomic.AddInt32(&n, affected.Count)
						util.Log.Printf("delete %d from worker %s", affected.Count, endpoint)
					}
				}
			}
		}(endpoint)
	}
	wg.Wait()
	return int(atomic.LoadInt32(&n))
}

func (sentinel *Sentinel) Search(query *types.TermQuery, onFlag uint64, offFlag uint64, orFlags []uint64) []*types.Document {
	endpoints := sentinel.hub.GetServiceEndpoints(INDEX_SERVICE)
	if len(endpoints) == 0 {
		return nil
	}
	docs := make([]*types.Document, 0, 1000)
	resultCh := make(chan *types.Document, 1000)
	wg := sync.WaitGroup{}
	wg.Add(len(endpoints))

	for _, endpoint := range endpoints {
		go func(endpoint string) {
			defer wg.Done()
			conn := sentinel.GetGrpcConn(endpoint)
			if conn != nil {
				client := NewIndexServiceClient(conn)
				result, err := client.Search(context.Background(), &SearchRequest{query, onFlag, offFlag, orFlags})
				if err != nil {
					util.Log.Printf("search from cluster failed: %s", err)
				} else {
					if len(result.Result) > 0 {
						util.Log.Printf("search %d doc from worker %s", len(result.Result), endpoint)
						for _, doc := range result.Result {
							resultCh <- doc
						}
					}
				}
			}
		}(endpoint)
	}

	receiveFinish := make(chan struct{})
	go func() {
		for {
			doc, ok := <-resultCh
			if !ok {
				break
			}
			docs = append(docs, doc)
		}
		receiveFinish <- struct{}{}
	}()
	wg.Wait()
	close(resultCh)
	<-receiveFinish
	return docs
}

func (sentinel *Sentinel) Count() int {
	var n int32
	endpoints := sentinel.hub.GetServiceEndpoints(INDEX_SERVICE)
	if len(endpoints) == 0 {
		return 0
	}
	wg := sync.WaitGroup{}
	wg.Add(len(endpoints))
	for _, endpoint := range endpoints {
		go func(endpoint string) {
			defer wg.Done()
			conn := sentinel.GetGrpcConn(endpoint)
			if conn != nil {
				client := NewIndexServiceClient(conn)
				affected, err := client.Count(context.Background(), new(CountRequest))
				if err != nil {
					util.Log.Printf("get doc count from worker %s failed: %s", endpoint, err)
				} else {
					if affected.Count > 0 {
						atomic.AddInt32(&n, affected.Count)
						util.Log.Printf("worker %s have %d documents", endpoint, affected.Count)
					}
				}
			}
		}(endpoint)
	}
	wg.Wait()
	return int(n)
}

// 关闭各个grpc client connection，关闭etcd client connection
func (sentinel *Sentinel) Close() (err error) {
	sentinel.connPool.Range(func(key, value any) bool {
		conn := value.(*grpc.ClientConn)
		err = conn.Close()
		return true
	})
	sentinel.hub.Close()
	return
}
