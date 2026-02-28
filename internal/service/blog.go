package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/amemiya02/hmdp-go/internal/constant"
	"github.com/amemiya02/hmdp-go/internal/global"
	"github.com/amemiya02/hmdp-go/internal/model/dto"
	"github.com/amemiya02/hmdp-go/internal/model/entity"
	"github.com/amemiya02/hmdp-go/internal/repository"
	"github.com/amemiya02/hmdp-go/internal/util"
	"github.com/redis/go-redis/v9"
)

type BlogService struct {
	BlogRepository   *repository.BlogRepository
	UserRepository   *repository.UserRepository
	FollowRepository *repository.FollowRepository
}

func NewBlogService() *BlogService {
	return &BlogService{
		BlogRepository:   repository.NewBlogRepository(),
		UserRepository:   repository.NewUserRepository(),
		FollowRepository: repository.NewFollowRepository(),
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

func (bs *BlogService) CreateBlog(c context.Context, blog *entity.Blog) *dto.Result {
	// 1.获取登录用户
	user := util.GetUser(c)
	if user == nil {
		return dto.Fail("请先登录！")
	}
	blog.UserID = user.ID
	// 2.保存探店笔记
	err := bs.BlogRepository.CreateBlog(c, blog)
	if err != nil {
		return dto.Fail(err.Error())
	}
	// 3.查询笔记作者的所有粉丝 select * from tb_follow where follow_user_id = ?
	follows, err := bs.FollowRepository.QueryFollowsByFollowUserId(c, user.ID)
	if err != nil {
		// 查询粉丝失败属于非核心链路错误，只打日志，不阻塞笔记发布的成功返回
		global.Logger.Error(fmt.Sprintf("查询粉丝列表失败: %v", err))
		return dto.OkWithData(blog.ID)
	}
	// 4.推送笔记id给所有粉丝
	for _, follow := range follows {
		// 4.1.获取粉丝id
		followerId := follow.UserID
		// 4.2.推送
		key := constant.FeedKey + strconv.FormatUint(followerId, 10)
		err = global.RedisClient.ZAdd(c, key, redis.Z{
			Score:  float64(time.Now().UnixMilli()),
			Member: blog.ID,
		}).Err()
		if err != nil {
			// 推送给某个粉丝失败，打个日志，继续给下一个人推（不要直接 return 报错）
			global.Logger.Error(fmt.Sprintf("向粉丝 [%d] 推送笔记 [%d] 失败: %v", followerId, blog.ID, err))
		}
	}

	return dto.OkWithData(blog.ID)
}

func (bs *BlogService) QueryBlogById(ctx context.Context, blogId uint64) *dto.Result {
	blog, err := bs.BlogRepository.QueryBlogById(ctx, blogId)
	if err != nil {
		return dto.Fail(err.Error())
	}
	bs.queryBlogUser(ctx, blog)
	bs.isBlogLiked(ctx, blog)
	return dto.OkWithData(blog)
}

// QueryMyBlog 查询当前用户的探店笔记
func (bs *BlogService) QueryMyBlog(ctx context.Context, current int) *dto.Result {
	userId := util.GetUserId(ctx)
	if userId == 0 {
		return dto.Fail("请先登录！")
	}
	// 调用 Repo 层查询数据库
	blogs, err := bs.BlogRepository.QueryBlogsByUserId(ctx, userId, current)
	if err != nil {
		return dto.Fail("查询失败，请稍后再试")
	}

	// 返回查询到的列表 (即使为空数组 [] 也是正常的，直接返回)
	return dto.OkWithData(blogs)
}

func (bs *BlogService) LikeBlog(ctx context.Context, blogId uint64) *dto.Result {
	userId := util.GetUserId(ctx)
	if userId == 0 {
		return dto.Fail("请先登录！")
	}
	// 拼接 Redis Key
	key := constant.BlogLikedKey + strconv.FormatUint(blogId, 10)
	member := strconv.FormatUint(userId, 10)
	// 1. 判断当前登录用户是否已经点赞
	// 注意：在 go-redis 中，如果查不到数据，会返回 redis.Nil 错误
	_, err := global.RedisClient.ZScore(ctx, key, member).Result()
	if errors.Is(err, redis.Nil) {
		// 2. 如果未点赞，可以点赞
		// 2.1 数据库点赞数 + 1
		rowsAffected, dbErr := bs.BlogRepository.UpdateBlogLikeCount(ctx, blogId, true)
		if dbErr != nil {
			return dto.Fail("点赞失败")
		}

		// 2.2 保存用户到 Redis 的 ZSet 集合 (数据库更新成功后才写 Redis)
		if rowsAffected > 0 {
			global.RedisClient.ZAdd(ctx, key, redis.Z{
				Score:  float64(time.Now().UnixMilli()), // 当前时间戳作为 Score，用于后续点赞排行榜按时间排序
				Member: member,
			})
		}
	} else if err == nil {
		// 3. 如果已点赞，取消点赞
		// 3.1 数据库点赞数 - 1
		rowsAffected, dbErr := bs.BlogRepository.UpdateBlogLikeCount(ctx, blogId, false)
		if dbErr != nil {
			return dto.Fail("取消点赞失败")
		}

		// 3.2 把用户从 Redis 的 ZSet 集合移除
		if rowsAffected > 0 {
			global.RedisClient.ZRem(ctx, key, member)
		}
	} else {
		// Redis 查询发生其他异常
		return dto.Fail("系统繁忙，请稍后再试")
	}

	return dto.Ok()
}

func (bs *BlogService) QueryBlogLikes(c context.Context, blogId uint64) *dto.Result {
	key := constant.BlogLikedKey + strconv.FormatUint(blogId, 10)
	// 1. 查询 top5 的点赞用户 zrange key 0 4
	top5, err := global.RedisClient.ZRange(c, key, 0, 4).Result()
	if err != nil {
		return dto.OkWithData([]*dto.UserDTO{})
	}
	// 如果没人点赞，直接返回空数组 []
	if len(top5) == 0 {
		return dto.OkWithData(make([]*dto.UserDTO, 0))
	}

	// 2. 解析出其中的用户 id
	ids := make([]uint64, 0, len(top5))
	for _, idStr := range top5 {
		id, _ := strconv.ParseUint(idStr, 10, 64)
		ids = append(ids, id)
	}

	// 3. 根据用户 id 查询用户，并保持传入时的排序顺序
	users, err := bs.UserRepository.QueryUsersByIdsWithOrder(c, ids)
	if err != nil {
		return dto.Fail("查询用户信息失败")
	}

	// 4. 将 entity 实体转化为 UserDTO
	userDTOs := make([]*dto.UserDTO, 0, len(users))
	for _, u := range users {
		userDTOs = append(userDTOs, &dto.UserDTO{
			ID:       u.ID,
			NickName: u.NickName,
			Icon:     u.Icon,
		})
	}

	// 5. 返回
	return dto.OkWithData(userDTOs)
}

func (bs *BlogService) QueryBlogByUserId(c context.Context, userId uint64, current int) *dto.Result {
	blogs, err := bs.BlogRepository.QueryBlogByUserId(c, userId, current)
	if err != nil {
		return dto.Fail(err.Error())
	}
	return dto.OkWithData(blogs)
}

// QueryBlogOfFollow 滚动分页查询关注推送的笔记
func (bs *BlogService) QueryBlogOfFollow(ctx context.Context, max int64, offset int) *dto.Result {
	// 1. 获取当前用户
	userId := util.GetUserId(ctx)
	if userId == 0 {
		return dto.Fail("请先登录！")
	}

	// REV代表从大到小 byscore代表不根据角标查询而是根据分数查询
	// 2. 查询收件箱 ZREVRANGEBYSCORE key max 0 LIMIT offset count
	key := constant.FeedKey + strconv.FormatUint(userId, 10)

	op := &redis.ZRangeBy{
		Max:    strconv.FormatInt(max, 10),
		Min:    "0", // 最小时间戳为0
		Offset: int64(offset),
		Count:  constant.ScrollResultPageSize,
	}

	// 使用 WithScores 带上分数一起查出来
	typedTuples, err := global.RedisClient.ZRevRangeByScoreWithScores(ctx, key, op).Result()
	if err != nil {
		return dto.Fail("查询动态失败")
	}

	// 3. 非空判断
	if len(typedTuples) == 0 {
		return dto.OkWithData(&dto.ScrollResult{
			List:    make([]*entity.Blog, 0),
			MinTime: 0,
			Offset:  0,
		})
	}

	// 4. 解析数据：blogId、minTime（时间戳）、offset
	ids := make([]uint64, 0, len(typedTuples))
	var minTime int64 = 0
	var os = 1

	for _, tuple := range typedTuples {
		// 4.1 获取 id (在 go-redis 中 member 默认是 string 类型，需要转换)
		idStr, ok := tuple.Member.(string)
		if ok {
			id, _ := strconv.ParseUint(idStr, 10, 64)
			ids = append(ids, id)
		}

		// 4.2 获取分数(时间戳) 并计算下一次的 offset
		curTime := int64(tuple.Score)
		// 因为返回的结果是按score从大到小排序的 所以从头开始遍历后 到最后一定是最小的时间
		if curTime == minTime {
			os++
		} else {
			// 因为数组有大到小的序 不一样的话 curTime 肯定小于当前的mintime 直接更新
			minTime = curTime
			os = 1 // 只要出现了新的更小的时间戳，就把 offset 重置为 1
		}
	}

	// 5. 根据 id 查询 blog，并保持 Redis 中的时间倒序
	blogs, err := bs.BlogRepository.QueryBlogsByIdsWithOrder(ctx, ids)
	if err != nil {
		return dto.Fail("查询动态详情失败")
	}

	// 6. 完善 blog 详情（查询发布者信息、当前用户是否点赞）
	for _, blog := range blogs {
		bs.queryBlogUser(ctx, blog)
		bs.isBlogLiked(ctx, blog)
	}

	// 7. 封装并返回
	scrollResult := &dto.ScrollResult{
		List:    blogs,
		MinTime: minTime, // 本次的最小时间戳 下次的最大值
		Offset:  os,      // 下次查询跳过几个score为mintime的
	}

	return dto.OkWithData(scrollResult)
}
