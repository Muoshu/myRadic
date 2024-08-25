package filter

import (
	"github.com/Muoshu/myRadic/demo"
	"github.com/Muoshu/myRadic/demo/video_search/common"
)

type ViewFilter struct {
}

func (ViewFilter) Apply(ctx *common.VideoSearchContext) {
	req := ctx.Request
	if req == nil {
		return
	}
	if req.ViewFrom >= req.ViewTo { //非法范围
		return
	}
	videos := make([]*demo.BiliVideo, 0, len(ctx.Videos))
	for _, video := range ctx.Videos {
		if video.View >= int32(req.ViewFrom) && video.View <= int32(req.ViewTo) {
			videos = append(videos, video)
		}
	}
	ctx.Videos = videos
}
