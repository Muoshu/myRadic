package reverse_index

import (
	"github.com/huandu/skiplist"
	farmhash "github.com/leemcloughlin/gofarmhash"
	"myRadic/types"
	"myRadic/util"
	"runtime"
	"sync"
)

// SkipListReverseIndex 倒排索引整体上是个map，map的value是一个SkipList
type SkipListReverseIndex struct {
	table *util.ConcurrentHashMap //分段map，并发安全
	locks []sync.RWMutex          //修改倒排索引时，相同的key需要去竞争同一把锁
}

func NewSkipListReverseIndex(docNum int) *SkipListReverseIndex {
	indexer := new(SkipListReverseIndex)
	indexer.table = util.NewConcurrentHashMap(runtime.NumCPU(), docNum)
	indexer.locks = make([]sync.RWMutex, 1000)
	return indexer
}

func (indexer *SkipListReverseIndex) getLock(key string) *sync.RWMutex {
	n := int(farmhash.Hash32WithSeed([]byte(key), 0))
	return &indexer.locks[n%len(indexer.locks)]
}

type SkipListValue struct {
	Id         string //业务Id
	BitFeature uint64
}

func (indexer *SkipListReverseIndex) Add(doc types.Document) {
	for _, keyword := range doc.Keywords {
		key := keyword.ToString()
		lock := indexer.getLock(key)
		lock.Lock()
		skpVal := SkipListValue{doc.Id, doc.BitsFeature}
		if val, ok := indexer.table.Get(key); ok {
			list := val.(*skiplist.SkipList)
			list.Set(doc.IntId, skpVal) //IntId作为SkipList的key，而value里则包含了业务侧的文档id和BitsFeature
		} else {
			list := skiplist.New(skiplist.Uint64)
			list.Set(doc.IntId, skpVal)
			indexer.table.Set(key, val)
		}
		lock.Unlock()

	}

}

func (indexer *SkipListReverseIndex) Delete(IntId uint64, keyword *types.Keyword) {
	key := keyword.ToString()
	lock := indexer.getLock(key)
	lock.Lock()
	if val, ok := indexer.table.Get(key); ok {
		list := val.(*skiplist.SkipList)
		list.Remove(IntId)
	}
	lock.Unlock()

}

// 多个跳表求交集
func IntersectionOfSkipList(lists ...*skiplist.SkipList) *skiplist.SkipList {
	if len(lists) == 0 {
		return nil
	}
	if len(lists) == 1 {
		return lists[0]
	}

	res := skiplist.New(skiplist.Uint64)
	//给每条SkipList分配一个指针，从前往后遍历
	currNodes := make([]*skiplist.Element, len(lists))
	for i, list := range lists {
		if list == nil || list.Len() == 0 {
			return nil
		}
		currNodes[i] = list.Front()
	}

	for {
		//此刻，哪个指针对应的值最大（最大者可能存在多个，所以用map）
		maxList := make(map[int]struct{}, len(currNodes))
		var maxValue uint64 = 0
		for i, node := range currNodes {
			if node.Key().(uint64) > maxValue {
				maxValue = node.Key().(uint64)
				maxList = map[int]struct{}{i: {}} //可以用一对大括号表示空结构体实例
			} else if node.Key().(uint64) == maxValue {
				maxList[i] = struct{}{}
			}
		}
		//所有node的值都一样大，则新诞生一个交集
		if len(maxList) == len(currNodes) {
			res.Set(currNodes[0].Key(), currNodes[0].Value)
			for i, node := range currNodes {
				if node.Next() == nil {
					return res
				}
				currNodes[i] = node.Next()
			}
		} else {
			for i, node := range currNodes {
				//值大的不动，小的往后移
				if _, ok := maxList[i]; !ok {
					if node.Next() == nil {
						return res
					}
					currNodes[i] = node.Next()

				}
			}
		}
	}
}

// 多个跳表求并集
func UnionSetOfSkipList(lists ...*skiplist.SkipList) *skiplist.SkipList {
	if len(lists) == 0 {
		return nil
	}
	if len(lists) == 1 {
		return lists[0]
	}

	res := skiplist.New(skiplist.Uint64)
	keySet := make(map[any]struct{}, 1000)
	for _, list := range lists {
		if list == nil {
			continue
		}
		node := list.Front()
		for node != nil {
			if _, ok := keySet[node.Key()]; !ok {
				res.Set(node.Key(), node.Value)
				keySet[node.Key()] = struct{}{}
			}
			node = node.Next()
		}
	}
	return res
}

func (indexer SkipListReverseIndex) FilterByBits(bits uint64, onFlag uint64, offFlag uint64, orFlags []uint64) bool {
	//onFlag所有bit必须全部命中
	if bits&onFlag != onFlag {
		return false
	}
	//offFlag所有bit必须全部不命中
	if bits&offFlag != 0 {
		return false
	}
	//多个orFlags必须全部命中
	for _, orFlag := range orFlags {
		if orFlag > 0 && bits&orFlag <= 0 { //单个orFlag只人有一个bit命中即可
			return false
		}
	}
	return true
}

func (indexer SkipListReverseIndex) search(q *types.TermQuery, onFlag uint64, offFlag uint64, orFlags []uint64) *skiplist.SkipList {
	if q.Keyword != nil {
		keyword := q.Keyword.ToString()
		if val, ok := indexer.table.Get(keyword); ok {
			res := skiplist.New(skiplist.Uint64)
			list := val.(*skiplist.SkipList)
			node := list.Front()
			for node != nil {
				intId := node.Key().(uint64)
				skpVal, _ := node.Value.(SkipListValue)
				flag := skpVal.BitFeature
				if intId > 0 && indexer.FilterByBits(flag, onFlag, offFlag, orFlags) { //确保有效元素都大于0
					res.Set(intId, skpVal)
				}
				node = node.Next()
			}
			return res
		}
	} else if len(q.Must) > 0 {
		res := make([]*skiplist.SkipList, 0, len(q.Must))
		for _, q := range q.Must {
			res = append(res, indexer.search(q, onFlag, offFlag, orFlags))
		}
		return IntersectionOfSkipList(res...)
	} else if len(q.Should) > 0 {
		res := make([]*skiplist.SkipList, 0, len(q.Should))
		for _, q := range q.Should {
			res = append(res, indexer.search(q, onFlag, offFlag, orFlags))
		}
		return UnionSetOfSkipList(res...)
	}
	return nil
}

func (indexer SkipListReverseIndex) Search(query *types.TermQuery, onFlag uint64, offFlag uint64, orFlags []uint64) []string {
	res := indexer.search(query, onFlag, offFlag, orFlags)
	if res == nil {
		return nil
	}
	arr := make([]string, 0, res.Len())
	node := res.Front()
	for node != nil {
		skpVal, _ := node.Value.(SkipListValue)
		arr = append(arr, skpVal.Id)
		node = node.Next()
	}
	return arr
}
