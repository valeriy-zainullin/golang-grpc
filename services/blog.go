package blog

import (
	"context"
	"errors"
	"sync"

	proto "main/gen"

	"google.golang.org/grpc"
)

type Blog struct {
	proto.BlogServer
	mutex      sync.Mutex
	posts      []*proto.Post
	nextPostId uint64
}

func (blog *Blog) GetPosts(page *proto.Page, posts grpc.ServerStreamingServer[proto.Post]) error {
	blog.mutex.Lock()
	defer blog.mutex.Unlock()

	for _, post := range blog.posts {
		if page.Offset <= post.Id.Value && post.Id.Value < page.Offset+page.Limit {
			posts.Send(post)
		}
	}

	return nil
}

func (blog *Blog) CreatePost(_ context.Context, post *proto.Post) (*proto.CreatePostResult, error) {
	blog.mutex.Lock()
	defer blog.mutex.Unlock()

	blog.posts = append(blog.posts, post)
	post.Id.Value = blog.nextPostId
	blog.nextPostId += 1

	return &proto.CreatePostResult{Success: true}, nil
}

var CouldNotFindPost = errors.New("Post with the specified id is not found")

func (blog *Blog) findPostById(id *proto.PostId) (uint64, error) {
	for idx, post := range blog.posts {
		if post.Id.Value == id.Value {
			return uint64(idx), nil
		}
	}
	return 0, CouldNotFindPost
}

func (blog *Blog) EditPost(_ context.Context, post *proto.Post) (*proto.EditPostResult, error) {
	blog.mutex.Lock()
	defer blog.mutex.Unlock()

	storedIdx, err := blog.findPostById(post.Id)
	if err != nil {
		return &proto.EditPostResult{Success: false}, err
	}

	blog.posts[storedIdx] = post

	return &proto.EditPostResult{Success: true}, nil
}

func swap[T any](left *T, right *T) {
	tmp := left
	*left = *right
	*right = *tmp
}

func (blog *Blog) DeletePost(_ context.Context, postId *proto.PostId) (*proto.DeletePostResult, error) {
	blog.mutex.Lock()
	defer blog.mutex.Unlock()

	storedIdx, err := blog.findPostById(postId)
	if err != nil {
		return &proto.DeletePostResult{Success: false}, err
	}

	swap(blog.posts[storedIdx], blog.posts[len(blog.posts)-1])

	blog.posts = blog.posts[:len(blog.posts)-1]

	return &proto.DeletePostResult{Success: true}, nil
}

func (blog *Blog) LikePost(_ context.Context, postId *proto.PostId) (*proto.LikePostResult, error) {
	blog.mutex.Lock()
	defer blog.mutex.Unlock()

	storedIdx, err := blog.findPostById(postId)
	if err != nil {
		return &proto.LikePostResult{Success: false}, err
	}

	/* No bound checks and deduplication yet! */
	blog.posts[storedIdx].NumLiked += 1

	return &proto.LikePostResult{Success: true}, nil
}

func (blog *Blog) UnlikePost(_ context.Context, postId *proto.PostId) (*proto.UnlikePostResult, error) {
	blog.mutex.Lock()
	defer blog.mutex.Unlock()

	storedIdx, err := blog.findPostById(postId)
	if err != nil {
		return &proto.UnlikePostResult{Success: false}, err
	}

	/* No bound checks and deduplication yet! */
	blog.posts[storedIdx].NumLiked -= 1

	return &proto.UnlikePostResult{Success: true}, nil
}
