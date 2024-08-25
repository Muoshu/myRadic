package kvdb

import (
	"errors"
	"github.com/Muoshu/myRadic/util"
	"github.com/dgraph-io/badger/v4"
	"os"
	"path"
	"sync/atomic"
)

type Badger struct {
	db   *badger.DB
	path string
}

func (b *Badger) WithDataPath(path string) *Badger {
	b.path = path
	return b
}

func (b *Badger) Open() error {
	dataDir := b.GetDbPath()
	if err := os.MkdirAll(path.Dir(dataDir), os.ModePerm); err != nil {
		return err
	}
	option := badger.DefaultOptions(dataDir).WithNumVersionsToKeep(1).WithLoggingLevel(badger.ERROR)
	db, err := badger.Open(option)
	if err != nil {
		return err
	}
	b.db = db
	return nil
}

func (b *Badger) GetDbPath() string {
	return b.path
}

// (超出接口规范，额外多出来的方法)
func (b *Badger) CheckAndGC() {
	lsmSize1, vlogSize1 := b.db.Size()
	for {
		if err := b.db.RunValueLogGC(0.5); err == badger.ErrNoRewrite || err == badger.ErrRejected {
			break
		}
	}
	lsmSize2, vlogSize2 := b.db.Size()
	if vlogSize2 < vlogSize1 {
		util.Log.Printf("badger before GC, LSM %d, vlog %d. after GC, LSM %d, vlog %d", lsmSize1, vlogSize1, lsmSize2, vlogSize2)
	} else {
		util.Log.Printf("collect zero garbage")
	}
}

func (b *Badger) Set(k, v []byte) error {
	err := b.db.Update(func(txn *badger.Txn) error {
		return txn.Set(k, v)
	})
	return err
}
func (b *Badger) BatchSet(keys, values [][]byte) error {
	if len(keys) != len(values) {
		return errors.New("key value not the same length")
	}
	var err error
	txn := b.db.NewTransaction(true)
	for i, key := range keys {
		value := values[i]
		//duration := time.Hour * 87600
		//util.util.Log.Debugf("duration",duration)
		if err = txn.Set(key, value); err != nil {
			_ = txn.Commit() //发生异常时就提交老事务，然后开一个新事务，重试set
			txn = b.db.NewTransaction(true)
			_ = txn.Set(key, value)
		}
	}
	txn.Commit()
	return err
}

func (b *Badger) Get(k []byte) ([]byte, error) {
	var res []byte
	err := b.db.View(func(txn *badger.Txn) error { //db.View相当于打开了一个读写事务:db.NewTransaction(true)。用db.Update的好处在于不用显式调用Txn.Discard()了
		item, err := txn.Get(k)
		if err != nil {
			return err
		}
		err = item.Value(func(val []byte) error {
			res = val
			return nil
		})
		return err
	})
	return res, err
}

func (b *Badger) BatchGet(keys [][]byte) ([][]byte, error) {
	var err error
	txn := b.db.NewTransaction(false) //只读事务
	values := make([][]byte, len(keys))
	for i, key := range keys {
		var item *badger.Item
		item, err = txn.Get(key)
		if err == nil {
			//buffer := make([]byte, badgerOptions.ValueLogMaxEntries)
			var ival []byte
			//ival, err = item.ValueCopy(buffer)
			err = item.Value(func(val []byte) error {
				ival = val
				return nil
			})
			if err == nil {
				values[i] = ival
			} else { //拷贝失败
				values[i] = []byte{} //拷贝失败就把value设为空数组
			}
		} else { //读取失败
			values[i] = []byte{}              //读取失败就把value设为空数组
			if err != badger.ErrKeyNotFound { //如果真的发生异常，则开一个新事务继续读后面的key
				txn.Discard()
				txn = b.db.NewTransaction(false)
			}
		}
	}
	txn.Discard() //只读事务调Discard就可以了，不需要调Commit。Commit内部也会调Discard
	return values, err
}

func (b *Badger) Delete(k []byte) error {
	err := b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(k)

	})
	return err
}

func (b *Badger) BatchDelete(keys [][]byte) error {
	var err error
	txn := b.db.NewTransaction(true)
	for _, key := range keys {
		if err = txn.Delete(key); err != nil {
			_ = txn.Commit() //发生异常时就提交老事务，然后开一个新事务，重试delete
			txn = b.db.NewTransaction(true)
			_ = txn.Delete(key)
		}
	}
	txn.Commit()
	return err
}

func (b *Badger) Has(k []byte) bool {
	var exists = false
	b.db.View(func(txn *badger.Txn) error { //db.View相当于打开了一个读写事务:db.NewTransaction(true)。用db.Update的好处在于不用显式调用Txn.Discard()了
		_, err := txn.Get(k)
		if err != nil {
			return err
		} else {
			exists = true //没有任何异常发生，则认为k存在。如果k不存在会发生ErrKeyNotFound
		}
		return err
	})
	return exists
}

func (b *Badger) IterDB(fn func(k, v []byte) error) int64 {
	var total int64
	b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			var ival []byte
			//var err error
			//buffer := make([]byte, badgerOptions.ValueLogMaxEntries)
			//ival, err = item.ValueCopy(buffer)

			err := item.Value(func(val []byte) error {
				ival = val
				return nil
			})

			if err != nil {
				continue
			}
			if err := fn(key, ival); err == nil {
				atomic.AddInt64(&total, 1)
			}
		}
		return nil
	})
	return atomic.LoadInt64(&total)
}

// IterKey 只遍历key。key是全部存在LSM tree上的，只需要读内存，所以很快
func (b *Badger) IterKey(fn func(k []byte) error) int64 {
	var total int64
	b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false //只需要读key，所以把PrefetchValues设为false
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			k := item.Key()
			if err := fn(k); err == nil {
				atomic.AddInt64(&total, 1)
			}
		}
		return nil
	})
	return atomic.LoadInt64(&total)
}

// Close 把内存中的数据flush到磁盘，同时释放文件锁。如果没有close，再open时会丢失很多数据
func (b *Badger) Close() error {
	return b.db.Close()
}
