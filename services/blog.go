package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	proto "main/gen"
	"main/models"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type Blog struct {
	proto.BlogServer

	posts      []*proto.Post
	nextPostId uint64

	cacheMutex sync.RWMutex
	db         *gorm.DB
	rdb        *redis.Client
}

func (blog *Blog) SetDB(db *gorm.DB) {
	if blog.db != nil {
		log.Fatalf("Cannot change Blog service db once set")
	}

	blog.db = db
}

func (blog *Blog) SetRedisDB(rdb *redis.Client) {
	if blog.rdb != nil {
		log.Fatalf("Cannot change Blog service db once set")
	}

	blog.rdb = rdb
}

var missingUserId = status.Error(codes.Unauthenticated, "user id should be provided in user_id header")
var multipleIds = status.Error(codes.InvalidArgument, "multiple user ids provided")

func makeInvalidUserIdErr(err error) error {
	return status.Error(codes.InvalidArgument, fmt.Sprintf("user id is invalid: %v", err))
}

func ExtractUserId(ctx context.Context) (uint64, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0, missingUserId
	}

	ids := md["user_id"]
	if len(ids) == 0 {
		return 0, missingUserId
	} else if len(ids) > 1 {
		return 0, multipleIds
	}

	id, err := strconv.ParseUint(ids[0], 10, 64)

	if err != nil {
		return 0, makeInvalidUserIdErr(err)
	}

	return id, nil
}

func GetNumLikedKey(post *models.Post) string {
	return fmt.Sprintf("post_%d_num_liked", post.Id)
}

const cachedPostsKey = "posts_last_N"
const numCachedPosts = 100

func (blog *Blog) GetPostsRaw(page *proto.Page) ([]*proto.Post, error) {
	var dbPosts []models.Post

	// TODO: test negative offset and limit.
	result := blog.db.Order("id DESC").Offset(int(page.Offset)).Limit(int(page.Limit)).Find(&dbPosts)
	if result.Error != nil {
		return nil, result.Error
	}

	if dbPosts == nil {
		log.Fatalf("dbPosts is a nil slice")
	}

	messages := make([]*proto.Post, 0)

	for _, post := range dbPosts {
		message := proto.Post{
			Id:       &proto.PostId{},
			AuthorId: &proto.UserId{},
		}
		post.FillGrpcMessage(&message)

		numLikedKey := GetNumLikedKey(&post)

		cmd := blog.rdb.Get(context.Background(), numLikedKey)
		if cmd.Err() != redis.Nil {
			numLiked, err := cmd.Uint64()
			if err != nil {
				return nil, err
			}

			message.NumLiked = numLiked
		} else {
			message.NumLiked = post.GetNumLiked(blog.db)
			cmd := blog.rdb.Set(context.Background(), numLikedKey, message.NumLiked, 600*time.Second)

			if cmd.Err() != nil {
				log.Printf("failed to add redis key-value pair: %v", cmd.Err())
			}
		}

		messages = append(messages, &message)
	}

	return messages, nil
}

func (blog *Blog) CachePosts() error {
	// Posts must not change while updating cache entry.
	//   Also likes should not appear and other things.
	// The reason is that there will be consistency issues
	//   otherwise. Likes calculated for wrong posts and etc.
	// Only cache calculation and invalidation are blocking.
	//   Blog has RWMutex and reading operations take reading lock
	//   (reading operations do not block each other),
	//   cache updates blog operations take writing locks.

	blog.cacheMutex.Lock()
	defer blog.cacheMutex.Unlock()

	posts, err := blog.GetPostsRaw(&proto.Page{Offset: 0, Limit: numCachedPosts})
	if err != nil {
		return err
	}

	encoded, err := json.Marshal(posts)

	if err != nil {
		return err
	}

	// Set some duration, so that if there is an inconsistency between cache
	//   and db, it gets resolved.
	// Cached values should never be used for calculations that go into DB!
	//   Otherwise inconsistencies may become permanent.
	cmd := blog.rdb.Set(context.Background(), cachedPostsKey, encoded, 600*time.Second)

	if cmd.Err() != nil {
		log.Printf("failed to add redis key-value pair: %v", cmd.Err())
	}

	return nil
}

func (blog *Blog) InvalidatePostCache() {
	blog.cacheMutex.Lock()
	defer blog.cacheMutex.Unlock()

	cmd := blog.rdb.Del(context.Background(), cachedPostsKey)
	if cmd.Err() != nil {
		// Ok, will be invalidated based on expiration (after 10 mins).
		log.Printf("Failed to invalidate post cache: %v", cmd.Err())
	}
}

func (blog *Blog) GetPostsFromCache() ([]*proto.Post, error) {
	blog.cacheMutex.RLock()
	defer blog.cacheMutex.RUnlock()

	cmd := blog.rdb.Get(context.Background(), cachedPostsKey)

	if cmd.Err() == nil {
		encoded, err := cmd.Bytes()
		if err != nil {
			return nil, err
		}

		posts := []*proto.Post{}
		err = json.Unmarshal(encoded, &posts)
		if err != nil {
			return nil, err
		}

		return posts, nil
	} else if cmd.Err() == redis.Nil {
		// Fine, not cached. Cache later.
		return nil, nil
	} else {
		// Some hard error, report!
		return nil, cmd.Err()
	}
}

func (blog *Blog) GetPosts(page *proto.Page, messages grpc.ServerStreamingServer[proto.Post]) error {
	/*
	 * First 10 pages of posts are cached.
	 * A user may be unregistered, then use plain cache.
	 *
	 * For registered users cache liked flags for the first
	 * 100 posts. It should be cheap.
	 *
	 * Number of likes is cached for the same 100 posts (first 10 pages).
	 *
	 * So it is 10ms x 3 = 30ms of redis cache request roundtrip.
	 * For uncached case it is more, but it is quite rare.
	 */

	result := []*proto.Post{}

	cachedPosts, err := blog.GetPostsFromCache()
	if err != nil {
		log.Printf("failed to get posts from cache: %v", err)
	} else if cachedPosts == nil {
		// Cache posts for later
		err := blog.CachePosts()
		if err != nil {
			log.Printf("failed to cache posts: %v", err)
		}
	} else {
		// Try to fill at least part of the requested page.

		if int(page.Offset) < len(cachedPosts) && len(cachedPosts) > 0 {
			numFilled := max(
				min(
					min(page.Limit, numCachedPosts-page.Offset),
					int32(len(cachedPosts))),
				0,
			)
			result = append(result, cachedPosts[page.Offset:page.Offset+numFilled]...)
			page.Offset += numFilled
			page.Limit -= numFilled
		}
	}

	// Old, plain, known db query of those posts...
	queriedPosts, err := blog.GetPostsRaw(page)
	if err != nil {
		return err
	}

	for _, post := range result {
		messages.Send(post)
	}

	for _, post := range queriedPosts {
		messages.Send(post)
	}

	return nil
}

func (blog *Blog) CreatePost(context context.Context, message *proto.Post) (*proto.CreatePostResult, error) {
	curUserId, err := ExtractUserId(context)
	if err != nil {
		return &proto.CreatePostResult{Success: false}, err
	}

	_, err = models.FindUserById(blog.db, curUserId)
	if err != nil {
		return &proto.CreatePostResult{Success: false, CreatedId: 0}, err
	}

	post := models.PostFromGrpcMessage(message)

	post.UserId = curUserId

	err = post.Create(blog.db)

	if err != nil {
		return &proto.CreatePostResult{Success: false, CreatedId: 0}, err
	}

	blog.InvalidatePostCache()

	return &proto.CreatePostResult{Success: true, CreatedId: post.Id}, nil
}

var couldNotFindPost = status.Error(codes.NotFound, "post with the specified id is not found")
var notPostAuthorErr = status.Error(codes.PermissionDenied, "must be author to edit post")

func (blog *Blog) EditPost(context context.Context, message *proto.Post) (*proto.EditPostResult, error) {
	curUserId, err := ExtractUserId(context)
	if err != nil {
		return &proto.EditPostResult{Success: false}, err
	}

	_, err = models.FindUserById(blog.db, curUserId)
	if err != nil {
		return &proto.EditPostResult{Success: false}, err
	}

	post, err := models.FindPostById(blog.db, message.Id.Value)
	if err != nil {
		return &proto.EditPostResult{Success: false}, couldNotFindPost
	}

	if post.UserId != curUserId {
		return &proto.EditPostResult{Success: false}, notPostAuthorErr
	}

	messagePost := models.PostFromGrpcMessage(message)
	messagePost.Id = post.Id
	messagePost.UserId = curUserId

	err = messagePost.Save(blog.db)
	if err != nil {
		return &proto.EditPostResult{Success: false}, err
	}

	blog.InvalidatePostCache()

	return &proto.EditPostResult{Success: true}, nil
}

func (blog *Blog) DeletePost(context context.Context, message *proto.PostId) (*proto.DeletePostResult, error) {
	curUserId, err := ExtractUserId(context)
	if err != nil {
		return &proto.DeletePostResult{Success: false}, err
	}

	_, err = models.FindUserById(blog.db, curUserId)
	if err != nil {
		return &proto.DeletePostResult{Success: false}, err
	}

	post, err := models.FindPostById(blog.db, message.Value)
	if err != nil {
		return &proto.DeletePostResult{Success: false}, couldNotFindPost
	}

	if post.UserId != curUserId {
		return &proto.DeletePostResult{Success: false}, notPostAuthorErr
	}

	err = post.Delete(blog.db)
	if err != nil {
		return &proto.DeletePostResult{Success: false}, err
	}

	blog.InvalidatePostCache()

	return &proto.DeletePostResult{Success: true}, nil
}

func (blog *Blog) LikePost(context context.Context, message *proto.PostId) (*proto.LikePostResult, error) {
	curUserId, err := ExtractUserId(context)
	if err != nil {
		return &proto.LikePostResult{Success: false}, err
	}

	user, err := models.FindUserById(blog.db, curUserId)
	if err != nil {
		return &proto.LikePostResult{Success: false}, err
	}

	post, err := models.FindPostById(blog.db, message.Value)
	if err != nil {
		return &proto.LikePostResult{Success: false}, couldNotFindPost
	}

	// If like does not exist, save will create it.
	like := models.Like{PostId: post.Id, UserId: curUserId}
	err = like.Save(blog.db)

	if err != nil {
		return &proto.LikePostResult{Success: false}, err
	}

	// Invalidate number of likes for the post.
	key := GetNumLikedKey(&post)
	cmd := blog.rdb.Del(context, key)

	if cmd.Err() != nil {
		// Data was commited at this point, so only log failed
		// to invalidate a cache entry.
		log.Printf("Failed to invalidate a key in redis cache: %s, error: %v", key, cmd.Err())
	}

	// Invalidate number of likes for the post.
	key = GetCheckLikedKey(&post, &user)
	cmd = blog.rdb.Del(context, key)

	if cmd.Err() != nil {
		// Data was commited at this point, so only log failed
		// to invalidate a cache entry.
		log.Printf("Failed to invalidate a key in redis cache: %s, error: %v", key, cmd.Err())
	}

	blog.InvalidatePostCache()

	return &proto.LikePostResult{Success: true}, nil
}

func GetCheckLikedKey(post *models.Post, user *models.User) string {
	return fmt.Sprintf("post_%d__user_%d__liked", post.Id, user.Id)
}

func (blog *Blog) CheckLikedPost(context context.Context, message *proto.PostId) (*proto.CheckLikedPostResult, error) {
	curUserId, err := ExtractUserId(context)
	if err != nil {
		return &proto.CheckLikedPostResult{Success: false}, err
	}

	user, err := models.FindUserById(blog.db, curUserId)
	if err != nil {
		return &proto.CheckLikedPostResult{Success: false}, err
	}

	post, err := models.FindPostById(blog.db, message.Value)
	if err != nil {
		return &proto.CheckLikedPostResult{Success: false}, couldNotFindPost
	}

	checkLikedKey := GetCheckLikedKey(&post, &user)
	cmd := blog.rdb.Get(context, checkLikedKey)
	if cmd.Err() != redis.Nil {
		liked, err := cmd.Bool()
		if err != nil {
			return &proto.CheckLikedPostResult{Success: false}, err
		}

		return &proto.CheckLikedPostResult{Success: true, Liked: liked}, nil
	} else {
		like := models.Like{PostId: post.Id, UserId: curUserId}
		liked, err := like.Exists(blog.db)
		if err != nil {
			return &proto.CheckLikedPostResult{Success: false}, err
		}

		cmd := blog.rdb.Set(context, checkLikedKey, liked, 600*time.Second)

		if cmd.Err() != nil {
			log.Printf("failed to add redis key-value pair: %v", cmd.Err())
		}

		return &proto.CheckLikedPostResult{Success: true, Liked: liked}, nil
	}
}

func (blog *Blog) UnlikePost(context context.Context, message *proto.PostId) (*proto.UnlikePostResult, error) {
	curUserId, err := ExtractUserId(context)
	if err != nil {
		return &proto.UnlikePostResult{Success: false}, err
	}

	user, err := models.FindUserById(blog.db, curUserId)
	if err != nil {
		return &proto.UnlikePostResult{Success: false}, err
	}

	post, err := models.FindPostById(blog.db, message.Value)
	if err != nil {
		return &proto.UnlikePostResult{Success: false}, couldNotFindPost
	}

	like := models.Like{PostId: post.Id, UserId: curUserId}
	err = like.Delete(blog.db)

	if err != nil {
		return &proto.UnlikePostResult{Success: false}, err
	}

	// Invalidate number of likes for the post.
	key := GetNumLikedKey(&post)
	cmd := blog.rdb.Del(context, key)

	if cmd.Err() != nil {
		// Data was commited at this point, so only log failed
		// to invalidate a cache entry.
		log.Printf("Failed to invalidate a key in redis cache: %s, error: %v", key, cmd.Err())
	}

	// Invalidate number of likes for the post.
	key = GetCheckLikedKey(&post, &user)
	cmd = blog.rdb.Del(context, key)

	if cmd.Err() != nil {
		// Data was commited at this point, so only log failed
		// to invalidate a cache entry.
		log.Printf("Failed to invalidate a key in redis cache: %s, error: %v", key, cmd.Err())
	}

	blog.InvalidatePostCache()

	return &proto.UnlikePostResult{Success: true}, nil
}
