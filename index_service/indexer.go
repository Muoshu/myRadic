package index_service

import (
	"bytes"
	"encoding/gob"
	"errors"
	"github.com/Muoshu/myRadic/internal/kvdb"
	reverseindex "github.com/Muoshu/myRadic/internal/reverse_index"
	"github.com/Muoshu/myRadic/types"
	"github.com/Muoshu/myRadic/util"
	"strings"
	"sync/atomic"
)

// 外观Facade模式。把正排和倒排2个子系统封装到了一起
type Indexer struct {
	forwardIndex kvdb.IKeyValueDB
	reverseIndex reverseindex.IReverseIndexer
	maxIntId     uint64
}

func (indexer *Indexer) Init(DocNumEstimate int, dbType int, dataDir string) error {
	db, err := kvdb.GetKvDb(dbType, dataDir)
	if err != nil {
		return err
	}
	indexer.forwardIndex = db
	indexer.reverseIndex = reverseindex.NewSkipListReverseIndex(DocNumEstimate)
	return nil
}

// LoadFromIndexFile 系统重启时，直接从索引文件里加载数据,其中v是document序列化后的字节流
func (indexer *Indexer) LoadFromIndexFile() int {
	reader := bytes.NewReader([]byte{})
	n := indexer.forwardIndex.IterDB(func(k, v []byte) error {
		reader.Reset(v)
		decoder := gob.NewDecoder(reader)
		var doc types.Document
		err := decoder.Decode(&doc)
		if err != nil {
			util.Log.Printf("gob decode document failed：%s", err)
			return err
		}
		indexer.reverseIndex.Add(doc)
		return nil
	})
	util.Log.Printf("load %d data from forward index %s", n, indexer.forwardIndex.GetDbPath())
	return int(n)
}

// 关闭索引
func (indexer *Indexer) Close() error {
	return indexer.forwardIndex.Close()
}

func (indexer *Indexer) DeleteDoc(docId string) int {
	n := 0
	forwardKey := []byte(docId)
	//先读正排索引，得到IntId和Keywords
	docBytes, err := indexer.forwardIndex.Get(forwardKey)
	if err == nil {
		reader := bytes.NewReader([]byte{})
		if len(docBytes) > 0 {
			n = 1
			reader.Reset(docBytes)
			decoder := gob.NewDecoder(reader)
			var doc types.Document
			err := decoder.Decode(&doc)
			if err == nil {
				// 遍历每一个keyword，从倒排索引上删除
				for _, keyword := range doc.Keywords {
					indexer.reverseIndex.Delete(doc.IntId, keyword)
				}
			}
		}
	}
	indexer.forwardIndex.Delete(forwardKey)
	return n
}

// 向索引中添加(亦是更新)文档(如果已存在，会先删除)
func (indexer *Indexer) AddDoc(doc types.Document) (int, error) {
	docId := strings.TrimSpace(doc.Id)
	if len(docId) == 0 {
		return 0, errors.New("doc id is empty")
	}
	//先从正排和倒排索引上将docId删除
	indexer.DeleteDoc(docId)
	//写入索引时自动为文档生成IntId
	doc.IntId = atomic.AddUint64(&indexer.maxIntId, 1)

	//写入正排索引
	var value bytes.Buffer
	encoder := gob.NewEncoder(&value)
	if err := encoder.Encode(doc); err != nil {
		return 0, err
	}
	indexer.forwardIndex.Set([]byte(docId), value.Bytes())

	//写入倒排索引
	indexer.reverseIndex.Add(doc)
	return 1, nil
}

// 检索，返回文档列表
func (indexer *Indexer) Search(query *types.TermQuery, onFlag uint64, offFlag uint64, orFlags []uint64) []*types.Document {
	docIds := indexer.reverseIndex.Search(query, onFlag, offFlag, orFlags)
	if len(docIds) == 0 {
		return nil
	}
	keys := make([][]byte, 0, len(docIds))
	for _, docId := range docIds {
		keys = append(keys, []byte(docId))
	}
	docs, err := indexer.forwardIndex.BatchGet(keys)
	if err != nil {
		util.Log.Printf("read kvdb failed: %s", err)
		return nil
	}
	result := make([]*types.Document, 0, len(docs))
	reader := bytes.NewReader([]byte{})
	for _, docBytes := range docs {
		if len(docs) > 0 {
			reader.Reset(docBytes)
			var doc types.Document
			decoder := gob.NewDecoder(reader)
			err := decoder.Decode(&doc)
			if err == nil {
				result = append(result, &doc)
			}
		}
	}
	return result
}

func (indexer *Indexer) Count() int {
	n := 0
	indexer.forwardIndex.IterKey(func(k []byte) error {
		n++
		return nil
	})
	return n
}
