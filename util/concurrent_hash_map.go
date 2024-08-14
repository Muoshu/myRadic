package util

import (
	farmhash "github.com/leemcloughlin/gofarmhash"
	"golang.org/x/exp/maps"
	"sync"
)

// ConcurrentHashMap 自行实现支持并发读写的map。key是string，value是any
type ConcurrentHashMap struct {
	mps   []map[string]any //由多个小map构成
	seg   int              //小map的个数
	locks []sync.RWMutex   //每个小map配一把读写锁。避免全局只有一把锁，影响性能
	seed  uint32           //每次执行farmhash传统一的seed
}

func NewConcurrentHashMap(seg, cap int) *ConcurrentHashMap {
	mps := make([]map[string]any, seg)
	locks := make([]sync.RWMutex, seg)
	for i := 0; i < seg; i++ {
		mps[i] = make(map[string]any, cap/seg)
	}
	return &ConcurrentHashMap{
		mps:   mps,
		seg:   seg,
		seed:  0,
		locks: locks,
	}
}

func (m *ConcurrentHashMap) getSegIndex(key string) int {
	hash := int(farmhash.Hash32WithSeed([]byte(key), m.seed))
	return hash % m.seg
}

func (m *ConcurrentHashMap) Set(key string, val any) {
	index := m.getSegIndex(key)
	m.locks[index].Lock()
	defer m.locks[index].Unlock()
	m.mps[index][key] = val
}

func (m *ConcurrentHashMap) Get(key string) (any, bool) {
	index := m.getSegIndex(key)
	m.locks[index].RLock()
	defer m.locks[index].RUnlock()
	val, ok := m.mps[index][key]
	return val, ok
}

type MapEntry struct {
	Key   string
	Value any
}

// MapIterator 迭代器模式
type MapIterator interface {
	Next() *MapEntry
}

type ConcurrentHashMapIterator struct {
	cm       *ConcurrentHashMap
	keys     [][]string
	rowIndex int
	colIndex int
}

func (m *ConcurrentHashMap) CreateIterator() *ConcurrentHashMapIterator {
	keys := make([][]string, 0, len(m.mps))
	for _, mp := range m.mps {
		row := maps.Keys(mp)
		keys = append(keys, row)
	}
	return &ConcurrentHashMapIterator{
		cm:       m,
		keys:     keys,
		rowIndex: 0,
		colIndex: 0,
	}
}

func (iter *ConcurrentHashMapIterator) Next() *MapEntry {
	if iter.rowIndex >= len(iter.keys) {
		return nil
	}
	row := iter.keys[iter.rowIndex]
	if len(row) == 0 { //本行为空,即当前小map为空
		iter.rowIndex += 1
		return iter.Next()
	}
	key := row[iter.colIndex] //根据下标访问切片元素时，一定注意不要出现数组越界异常。即使下标为0，当切片为空时依然会出现数组越界异常
	val, _ := iter.cm.Get(key)
	if iter.colIndex >= len(row)-1 {
		iter.rowIndex += 1
		iter.colIndex = 0
	} else {
		iter.colIndex += 1
	}
	return &MapEntry{
		key,
		val,
	}
}
