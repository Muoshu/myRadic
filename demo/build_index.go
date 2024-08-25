package demo

import (
	"encoding/csv"
	"github.com/Muoshu/myRadic/index_service"
	"github.com/Muoshu/myRadic/types"
	"github.com/Muoshu/myRadic/util"
	"github.com/gogo/protobuf/proto"
	farmhash "github.com/leemcloughlin/gofarmhash"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// 把CSV文件中的视频信息全部写入索引。
// totalWorkers: 分布式环境中一共有几台index worker，workerIndex本机是第几台worker(从0开始编号)。单机模式下把totalWorkers置0即可
func BuildIndexFromFile(csvFile string, indexer index_service.IIndexer, totalWorkers, workerIndex int) {
	file, err := os.Open(csvFile)
	if err != nil {
		log.Printf("open file %s failed: %s", csvFile, err)
		return
	}
	defer file.Close()

	loc, _ := time.LoadLocation("Asia/Shanghai")
	reader := csv.NewReader(file)
	progress := 0
	for {
		record, err := reader.Read()
		if err != nil {
			if err != io.EOF {
				log.Printf("read record failed: %s", err)
			}
			break
		}
		if len(record) < 10 { //避免数组越界，发生panic
			continue
		}

		docId := strings.TrimPrefix(record[0], "https://www.bilibili.com/video/")

		//只用一部分的视频数据
		if totalWorkers > 0 && int(farmhash.Hash32WithSeed([]byte(docId), 0))%totalWorkers != workerIndex {
			continue
		}

		video := &BiliVideo{
			Id:     docId,
			Title:  record[1],
			Author: record[3],
		}

		if len(record[2]) > 4 {
			t, err := time.ParseInLocation("2006/1/2 15:4", record[2], loc)
			if err != nil {
				log.Printf("parse time %s failed: %s", record[2], err)
			} else {
				video.PostTime = t.Unix()
			}
		}

		n, _ := strconv.Atoi(record[4])
		video.View = int32(n)
		n, _ = strconv.Atoi(record[5])
		video.Like = int32(n)
		n, _ = strconv.Atoi(record[6])
		video.Coin = int32(n)
		n, _ = strconv.Atoi(record[7])
		video.Favorite = int32(n)
		n, _ = strconv.Atoi(record[8])
		video.Share = int32(n)
		keywords := strings.Split(record[9], ",")
		if len(keywords) > 0 {
			for _, word := range keywords {
				word = strings.TrimSpace(word)
				if len(word) > 0 {
					video.Keywords = append(video.Keywords, strings.ToLower(word)) //转小写
				}
			}
		}
		AddVideo2Index(video, indexer) //构建好BiliVideo实体，写入索引
		progress++
	}
	util.Log.Printf("add %d documents to index totally", progress)
}

// AddVideo2Index 把一条视频信息写入索引（可能是create，也可能是update）
// 实时更新索引时可调该函数
func AddVideo2Index(video *BiliVideo, indexer index_service.IIndexer) {
	doc := types.Document{Id: video.Id}
	bs, err := proto.Marshal(video)
	if err != nil {
		log.Printf("serielize video failed: %s", err)
		return
	}
	doc.Bytes = bs
	keywords := make([]*types.Keyword, 0, len(video.Keywords))
	for _, word := range video.Keywords {
		keywords = append(keywords, &types.Keyword{Field: "content", Word: strings.ToLower(word)})

	}
	doc.Keywords = keywords
	if len(video.Author) > 0 {
		keywords = append(keywords, &types.Keyword{Field: "author", Word: strings.ToLower(strings.TrimSpace(video.Author))})
	}
	doc.BitsFeature = GetClassBits(video.Keywords)
	indexer.AddDoc(doc)
}
