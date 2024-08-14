package test

import (
	"fmt"
	"math/rand"
	"myRadic/util"
	"strconv"
	"sync"
	"testing"
)

var conMp = util.NewConcurrentHashMap(8, 1000)
var synMp = sync.Map{}

func readConMap() {
	for i := 0; i < 10000; i++ {
		key := strconv.Itoa(int(rand.Int63()))
		conMp.Get(key)
	}
}

func writeConMap() {
	for i := 0; i < 10000; i++ {
		key := strconv.Itoa(int(rand.Int63()))
		conMp.Set(key, 1)
	}
}

func readSynMap() {
	for i := 0; i < 10000; i++ {
		key := strconv.Itoa(int(rand.Int63()))
		synMp.Load(key)
	}
}

func writeSynMap() {
	for i := 0; i < 10000; i++ {
		key := strconv.Itoa(int(rand.Int63()))
		synMp.Store(key, 1)
	}
}

func BenchmarkConMap(b *testing.B) {
	for i := 0; i < b.N; i++ {
		const P = 300
		wg := sync.WaitGroup{}
		wg.Add(2 * P)
		for i := 0; i < P; i++ { //300个协程一直读
			go func() {
				defer wg.Done()
				readConMap()
			}()
		}
		for i := 0; i < P; i++ { //300个协程一直写
			go func() {
				defer wg.Done()
				writeConMap()
				// time.Sleep(100 * time.Millisecond)   //写很少时速度差1倍，一直写时速度差3倍
			}()
		}
		wg.Wait()
	}
}

func BenchmarkSynMap(b *testing.B) {
	for i := 0; i < b.N; i++ {
		const P = 300
		wg := sync.WaitGroup{}
		wg.Add(2 * P)
		for i := 0; i < P; i++ { //300个协程一直读
			go func() {
				defer wg.Done()
				readSynMap()
			}()
		}
		for i := 0; i < P; i++ { //300个协程一直写
			go func() {
				defer wg.Done()
				writeSynMap()
				// time.Sleep(100 * time.Millisecond)
			}()
		}
		wg.Wait()
	}
}

func TestConcurrentHashMapIterator(t *testing.T) {
	for i := 0; i < 10; i++ {
		conMp.Set(strconv.Itoa(i), i)
	}
	iterator := conMp.CreateIterator()
	entry := iterator.Next()
	for entry != nil {
		fmt.Println(entry.Key, entry.Value)
		entry = iterator.Next()
	}
}
