package kvdb

import (
	"myRadic/util"
	"os"
	"strings"
)

// 几种常见的基于LSM-tree算法实现的KV数据库
const (
	BOLT = iota
	BADGER
)

type IKeyValueDB interface {
	Open() error       //初始化DB
	GetDbPath() string //获取存储数据的目录
	Set(k, v []byte) error
	BatchSet(keys, values [][]byte) error
	Get(k []byte) ([]byte, error)
	BatchGet(keys [][]byte) ([][]byte, error)
	Delete(k []byte) error
	BatchDelete(keys [][]byte) error
	Has(k []byte) bool
	IterDB(fun func(k, v []byte) error) int64 //遍历数据库，返回数据条数
	IterKey(fun func(k []byte) error) int64   //遍历所有的key，返回数据条数
	Close() error                             //把内存中的数据flush到磁盘，同时释放文件锁
}

// Factory工厂模式，把类的创建和使用分隔开。Get函数就是一个工厂，它返回产品的接口，即它可以返回各种各样的具体产品。
func GetKvDb(dbType int, path string) (IKeyValueDB, error) {
	paths := strings.Split(path, "/")
	//父路径
	parentPath := strings.Join(paths[:len(paths)-1], "/")

	info, err := os.Stat(parentPath)
	if os.IsNotExist(err) { //父路劲不存在泽创建
		util.Log.Printf("create dir %s", parentPath)
		os.MkdirAll(parentPath, os.ModePerm) //数字前的0或0o都表示八进制
	} else {
		if info.Mode().IsRegular() { //如果父路径是个普通文件，则把它删掉
			util.Log.Printf("%s is a regular file, will delete it", parentPath)
			os.Remove(parentPath)
		}
	}
	var db IKeyValueDB
	switch dbType {
	case BADGER:
		db = new(Badger).WithDataPath(path)
	default: //默认使用bolt
		db = new(Bolt).WithDataPath(path).WithBucket("radic") //Builder生成器模式
	}
	err = db.Open() //创建具体KVDB的细节隐藏在Open()函数里。在这里【创建类】
	return db, err

}
