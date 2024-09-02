package main

import (
	"github.com/Muoshu/myRadic/demo"
	"github.com/Muoshu/myRadic/demo/handler"
	"github.com/Muoshu/myRadic/index_service"
	"os"
	"os/signal"
	"syscall"
)

func WebServerInit(mode int) {
	switch mode {
	case 1:
		//单机索引
		standaloneIndexer := new(index_service.Indexer)
		if err := standaloneIndexer.Init(50000, dbType, *dbPath); err != nil {
			panic(err)
		}
		if *rebuildIndex {
			demo.BuildIndexFromFile(csvFile, standaloneIndexer, 0, 0)
		} else {
			//直接从正排索引文件里面加载
			standaloneIndexer.LoadFromIndexFile()
		}
		handler.Indexer = standaloneIndexer
	case 3:
		handler.Indexer = index_service.NewSentinel(etcdServers)
	default:
		panic("invalid mode")

	}
}

func WebServerTeardown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	handler.Indexer.Close() //接收到kill信号时关闭索引
	os.Exit(0)              //然后自杀
}

func WebServerMain(mode int) {
	go WebServerTeardown()
	WebServerInit(mode)
}
