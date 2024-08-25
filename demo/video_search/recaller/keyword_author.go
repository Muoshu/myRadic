package recaller

import (
	"github.com/Muoshu/myRadic/demo"
	"github.com/Muoshu/myRadic/demo/video_search/common"
	"github.com/Muoshu/myRadic/types"
	"github.com/gogo/protobuf/proto"
	"strings"
)

type KeywordAuthorRecaller struct {
}

func (KeywordAuthorRecaller) Recall(ctx *common.VideoSearchContext) []*demo.BiliVideo {
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
			query = query.And(types.NewTermQuery("content", word)) //满足关键词
		}
	}

	v := ctx.Ctx.Value(common.UN("user_name"))
	if v != nil {
		if author, ok := v.(string); ok {
			if len(author) > 0 {
				query = query.And(types.NewTermQuery("author", strings.ToLower(author)))
			}
		}
	}
	orFlags := []uint64{demo.GetClassBits(req.Classes)} //满足类别
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
