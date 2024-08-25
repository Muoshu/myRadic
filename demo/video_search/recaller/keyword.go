package recaller

import (
	"github.com/Muoshu/myRadic/demo"
	"github.com/Muoshu/myRadic/demo/video_search/common"
	"github.com/Muoshu/myRadic/types"
	"github.com/gogo/protobuf/proto"
	"strings"
)

type KeywordRecaller struct {
}

func (KeywordRecaller) Recall(ctx *common.VideoSearchContext) []*demo.BiliVideo {
	req := ctx.Request
	if req == nil {
		return nil
	}
	indexer := ctx.Indexer
	if indexer == nil {
		return nil
	}
	keywords := req.Keywords
	query := new(types.TermQuery)
	if len(keywords) > 0 {
		for _, word := range keywords {
			//满足关键词
			query = query.And(types.NewTermQuery("content", word))
		}
	}

	if len(req.Author) > 0 {
		// 满足作者
		query = query.And(types.NewTermQuery("author", strings.ToLower(req.Author)))
	}
	//满足类别
	orFlags := []uint64{demo.GetClassBits(req.Classes)}
	docs := indexer.Search(query, 0, 0, orFlags)
	videos := make([]*demo.BiliVideo, 0, len(docs))
	for _, doc := range docs {
		var video demo.BiliVideo
		if err := proto.Unmarshal(doc.Bytes, &video); err == nil {
			videos = append(videos, &video)
		}
	}
	return videos
}
