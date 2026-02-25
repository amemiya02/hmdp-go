package service

import (
	"context"
	"errors"
	"strconv"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/repository"
	"github.com/amemiya02/hmdp-go/internal/util"
	"github.com/redis/go-redis/v9"
)

type BlogService struct {
	BlogRepository *repository.BlogRepository
	UserRepository *repository.UserRepository
}

func NewBlogService() *BlogService {
	return &BlogService{
		BlogRepository: repository.NewBlogRepository(),
		UserRepository: repository.NewUserRepository(),
	}
}

func (bs *BlogService) QueryHotBlog(c context.Context, current int) *dto.Result {
	blogs, err := bs.BlogRepository.QueryHotBlog(c, current)
	if err != nil {
		return dto.Fail(err.Error())
	}
	// 3. 循环完善博客信息 (查用户、查点赞状态)
	for _, blog := range blogs {
		bs.queryBlogUser(c, blog)
		bs.isBlogLiked(c, blog)
	}

	// 4. 返回结果
	return dto.OkWithData(blogs)
}

// queryBlogUser 完善博客的作者信息
func (bs *BlogService) queryBlogUser(ctx context.Context, blog *entity.Blog) {
	userId := blog.UserID
	user, _ := bs.UserRepository.FindUserById(ctx, userId)
	// 查到之后，给 blog 的附属字段赋值，例如：
	blog.Name = user.NickName
	blog.Icon = user.Icon
}

// isBlogLiked 判断当前登录用户是否点赞过该博客
func (bs *BlogService) isBlogLiked(ctx context.Context, blog *entity.Blog) {
	// 1. 获取登录用户 ID
	userId := util.GetUserId(ctx)
	if userId == 0 {
		// 用户未登录，无需查询是否点赞 (Go 里面布尔值默认就是 false，所以直接 return 即可)
		return
	}

	// 2. 拼接 Redis Key
	key := constant.BlogLikedKey + strconv.FormatUint(blog.ID, 10)
	member := strconv.FormatUint(userId, 10)

	// 3. 去 Redis 的 ZSet 中查询该用户的 score
	// 对应 Java: stringRedisTemplate.opsForZSet().score(key, userId.toString())
	_, err := global.RedisClient.ZScore(ctx, key, member).Result()

	// 4. 判断结果
	if err == nil {
		// 没报错，说明在 ZSet 中找到了这个 userId，代表已经点赞过了
		blog.IsLike = true
	} else if errors.Is(err, redis.Nil) {
		// 经典的 go-redis 查不到数据的标志，代表没点赞
		blog.IsLike = false
	} else {
		// Redis 发生其他异常（如网络波动），为了不影响页面渲染，默认按没点赞处理
		blog.IsLike = false
	}
}
