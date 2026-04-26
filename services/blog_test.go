package services_test

import (
	"context"
	"fmt"
	proto "main/gen"
	"main/models"
	"main/services"
	"main/utils"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestUserIdExtraction(t *testing.T) {
	variants := []metadata.MD{
		metadata.Pairs(),
		metadata.Pairs("user_id", "abcd"),
		metadata.Pairs("user_id", "1"),
		metadata.Pairs("user_id", "1", "user_id", "1"),
		metadata.Pairs("user_id", "1", "user_id", "2"),
	}

	isError := []bool{
		true,
		true,
		false,
		true,
		true,
	}

	errTexts := []string{
		"rpc error: code = Unauthenticated desc = user id should be provided in user_id header",

		// Maybe it is bad to uncover we are using go as it is easier to find vulnerabilities for attackers...
		"rpc error: code = InvalidArgument desc = user id is invalid: strconv.ParseUint: parsing \"abcd\": invalid syntax",

		"",

		"rpc error: code = InvalidArgument desc = multiple user ids provided",
		"rpc error: code = InvalidArgument desc = multiple user ids provided",
	}

	for i, md := range variants {
		ctx := metadata.NewIncomingContext(context.Background(), md)
		_, err := services.ExtractUserId(ctx)

		if isError[i] {
			assert.Error(t, err)
			assert.Equal(t, errTexts[i], err.Error())
		} else {
			assert.Equal(t, nil, err)
		}
	}
}

type StreamReader[Res any] struct {
	grpc.ServerStreamingServer[Res]

	items []*Res
}

func (reader *StreamReader[Res]) Send(item *Res) error {
	reader.items = append(reader.items, item)
	return nil
}

func TestGetPosts(t *testing.T) {
	blog := services.Blog{}

	db := utils.ConnectToTestDB()
	utils.FillDB(db)
	blog.SetDB(db)

	rdb := utils.ConnectToTestRedis()
	blog.SetRedisDB(rdb)

	postReader := StreamReader[proto.Post]{}

	err := blog.GetPosts(&proto.Page{Offset: 0, Limit: 10}, &postReader)
	assert.Equal(t, nil, err)

	expectedPost := proto.Post{
		Id:       &proto.PostId{Value: 1},
		AuthorId: &proto.UserId{Value: 1},
		Body:     "Hello, world! This is the first post of this blog.",
	}

	assert.Equal(t, 1, len(postReader.items))
	assert.Equal(t, expectedPost.Id.Value, postReader.items[0].Id.Value)
	assert.Equal(t, expectedPost.AuthorId.Value, postReader.items[0].AuthorId.Value)
	assert.Equal(t, expectedPost.Body, postReader.items[0].Body)

	postReader = StreamReader[proto.Post]{}
	err = blog.GetPosts(&proto.Page{Offset: 1, Limit: 10}, &postReader)
	assert.Equal(t, nil, err)
	assert.Equal(t, 0, len(postReader.items))

	postReader = StreamReader[proto.Post]{}
	err = blog.GetPosts(&proto.Page{Offset: 1, Limit: 1}, &postReader)
	assert.Equal(t, nil, err)
	assert.Equal(t, 0, len(postReader.items))
}

func TestCreatePost(t *testing.T) {
	blog := services.Blog{}

	db := utils.ConnectToTestDB()
	utils.FillDB(db)
	blog.SetDB(db)

	rdb := utils.ConnectToTestRedis()
	blog.SetRedisDB(rdb)

	md := metadata.Pairs("user_id", "1")
	posts := []proto.Post{
		{
			Id:       &proto.PostId{Value: 1},
			AuthorId: &proto.UserId{Value: 1},
			Body:     "This post should not be created as there is a post with id 1",
		},
		{
			Id:       &proto.PostId{Value: 0},
			AuthorId: &proto.UserId{Value: 2},
			Body:     "This post should be created with author id replaced to current user",
		},
		{
			Id:       &proto.PostId{Value: 3},
			AuthorId: &proto.UserId{Value: 1},
			Body:     "This post should be created",
		},
		{
			Id:       &proto.PostId{Value: 0},
			AuthorId: &proto.UserId{Value: 1},
			Body:     "This post should be created and id returned in result",
		},
	}

	isError := []bool{
		true,
		false,
		false,
		false,
	}

	createdId := []uint64{
		0,
		2,
		3,
		4,
	}

	for i := range posts {
		ctx := metadata.NewIncomingContext(context.Background(), md)
		result, err := blog.CreatePost(ctx, &posts[i])

		subtestName := fmt.Sprintf("case %d", i)

		if isError[i] {
			assert.Error(t, err, subtestName)
			assert.NotEqual(t, nil, result, subtestName)
			assert.Equal(t, false, result.Success, subtestName)
		} else {
			assert.Equal(t, nil, err, subtestName)
			assert.NotEqual(t, nil, result, subtestName)
			assert.Equal(t, true, result.Success, subtestName)
			assert.Equal(t, createdId[i], result.CreatedId, subtestName)
		}
	}

	for _, id := range createdId {
		if id == 0 {
			continue
		}

		post := models.Post{Id: id}
		err := post.Delete(db)
		assert.Nil(t, err, "failed to cleanup a post in TestCreatePost")
	}
}

func TestLikeUnlikePost(t *testing.T) {
	blog := services.Blog{}

	db := utils.ConnectToTestDB()
	utils.FillDB(db)
	blog.SetDB(db)

	rdb := utils.ConnectToTestRedis()
	blog.SetRedisDB(rdb)

	postReader := StreamReader[proto.Post]{}
	err := blog.GetPosts(&proto.Page{Offset: 0, Limit: 1}, &postReader)
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(postReader.items))
	assert.Equal(t, uint64(0), postReader.items[0].NumLiked)

	md := metadata.Pairs("user_id", "1")

	ctx := metadata.NewIncomingContext(context.Background(), md)
	checkLikedResult, err := blog.CheckLikedPost(ctx, &proto.PostId{Value: 1})
	assert.Nil(t, err)
	assert.True(t, checkLikedResult.Success)
	assert.False(t, checkLikedResult.Liked)

	ctx = metadata.NewIncomingContext(context.Background(), md)
	likeResult, err := blog.LikePost(ctx, &proto.PostId{Value: 1})
	assert.Nil(t, err)
	assert.True(t, likeResult.Success)

	postReader = StreamReader[proto.Post]{}
	err = blog.GetPosts(&proto.Page{Offset: 0, Limit: 1}, &postReader)
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(postReader.items))
	assert.Equal(t, uint64(1), postReader.items[0].NumLiked)

	ctx = metadata.NewIncomingContext(context.Background(), md)
	checkLikedResult, err = blog.CheckLikedPost(ctx, &proto.PostId{Value: 1})
	assert.Nil(t, err)
	assert.True(t, checkLikedResult.Success)
	assert.True(t, checkLikedResult.Liked)

	ctx = metadata.NewIncomingContext(context.Background(), md)
	unlikeResult, err := blog.UnlikePost(ctx, &proto.PostId{Value: 1})
	assert.Nil(t, err)
	assert.True(t, unlikeResult.Success)

	postReader = StreamReader[proto.Post]{}
	err = blog.GetPosts(&proto.Page{Offset: 0, Limit: 1}, &postReader)
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(postReader.items))
	assert.Equal(t, uint64(0), postReader.items[0].NumLiked)

	ctx = metadata.NewIncomingContext(context.Background(), md)
	checkLikedResult, err = blog.CheckLikedPost(ctx, &proto.PostId{Value: 1})
	assert.Nil(t, err)
	assert.True(t, checkLikedResult.Success)
	assert.False(t, checkLikedResult.Liked)
}
