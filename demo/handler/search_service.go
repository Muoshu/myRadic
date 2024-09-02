package handler

import (
	"context"
	"github.com/Muoshu/myRadic/demo"
	"github.com/Muoshu/myRadic/demo/video_search"
	"github.com/Muoshu/myRadic/demo/video_search/common"
	"github.com/Muoshu/myRadic/index_service"
	"github.com/Muoshu/myRadic/types"
	"github.com/Muoshu/myRadic/util"
	"github.com/gin-gonic/gin"
	"github.com/gogo/protobuf/proto"
	"log"
	"net/http"
	"strings"
)

var Indexer index_service.IIndexer

func cleanKeyword(words []string) []string {
	keywords := make([]string, 0, len(words))
	for _, w := range words {
		word := strings.TrimSpace(strings.ToLower(w))
		if len(word) > 0 {
			keywords = append(keywords, word)
		}
	}
	return keywords
}

// 搜索接口
func Search(ctx *gin.Context) {
	var request demo.SearchRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Printf("bind request parameter failed: %s", err)
		ctx.String(http.StatusBadRequest, "invalid json")
		return
	}

	keywords := cleanKeyword(request.Keywords)
	if len(keywords) == 0 && len(request.Author) == 0 {
		ctx.String(http.StatusBadRequest, "关键词和作者不能同时为空")
		return
	}
	query := new(types.TermQuery)
	if len(keywords) > 0 {
		for _, word := range keywords {
			//满足关键词
			query = query.And(types.NewTermQuery("content", word))
		}
	}
	if len(request.Author) > 0 {
		query = query.And(types.NewTermQuery("author", strings.ToLower(request.Author)))
	}
	//满足类别
	orFlags := []uint64{demo.GetClassBits(request.Classes)}
	docs := Indexer.Search(query, 0, 0, orFlags)
	videos := make([]demo.BiliVideo, 0, len(docs))
	for _, doc := range docs {
		var video demo.BiliVideo
		if err := proto.Unmarshal(doc.Bytes, &video); err == nil {
			if video.View >= int32(request.ViewFrom) && (request.ViewTo <= 0 || video.View <= int32(request.ViewTo)) { //满足播放量的区间范围
				videos = append(videos, video)
			}
		}
	}
	util.Log.Printf("return %d videos", len(videos))
	ctx.JSON(http.StatusOK, videos) //把搜索结果以json形式返回给前端
}

// 搜索全站视频
func SearchAll(ctx *gin.Context) {
	var request demo.SearchRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Printf("bind request parameter failed: %s", err)
		ctx.String(http.StatusBadRequest, "invalid json")
		return
	}

	request.Keywords = cleanKeyword(request.Keywords)
	if len(request.Keywords) == 0 && len(request.Author) == 0 {
		ctx.String(http.StatusBadRequest, "关键词和作者不能同时为空")
		return
	}

	searchCtx := &common.VideoSearchContext{
		Ctx:     context.Background(),
		Request: &request,
		Indexer: Indexer,
	}
	searcher := video_search.NewAllVideoSearcher()
	videos := searcher.Search(searchCtx)
	ctx.JSON(http.StatusOK, videos) //把搜索结果以json形式返回给前端
}

// up主在后台搜索自己的视频
func SearchByAuthor(ctx *gin.Context) {
	var request demo.SearchRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Printf("bind request parameter failed: %s", err)
		ctx.String(http.StatusBadRequest, "invalid json")
		return
	}

	request.Keywords = cleanKeyword(request.Keywords)
	if len(request.Keywords) == 0 {
		ctx.String(http.StatusBadRequest, "关键词不能为空")
		return
	}

	userName, ok := ctx.Value("user_name").(string) //从gin.Context里取得userName
	if !ok || len(userName) == 0 {
		ctx.String(http.StatusBadRequest, "获取不到登录用户名")
		return
	}
	searchCtx := &common.VideoSearchContext{
		Ctx:     context.WithValue(context.Background(), common.UN("user_name"), userName), //把userName放到context里
		Request: &request,
		Indexer: Indexer,
	}
	searcher := video_search.NewUpVideoSearcher()
	videos := searcher.Search(searchCtx)

	ctx.JSON(http.StatusOK, videos) //把搜索结果以json形式返回给前端
}
