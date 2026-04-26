package models

import (
	proto "main/gen"

	"gorm.io/gorm"
)

type Post struct {
	Id uint64 `gorm:"primaryKey"`

	UserId    uint64
	Body      string
	CreatedAt uint64

	// This is purely for many-to-one relation.
	// Not queries from database unless explicitly made.
	// TODO: test not queried along with the Post by default in TestLike method.
	Likes []Like
}

func (post *Post) Create(db *gorm.DB) error {
	return db.Create(post).Error
}

func (post *Post) Save(db *gorm.DB) error {
	return db.Save(post).Error
}

func (post *Post) Delete(db *gorm.DB) error {
	return db.Delete(post).Error
}

func (post *Post) GetNumLiked(db *gorm.DB) uint64 {
	result := db.Model(post).Association("Likes").Count()
	return uint64(result)
}

func FindPostById(db *gorm.DB, id uint64) (Post, error) {
	var post Post
	result := db.First(&post, id)
	return post, result.Error
}

func (post *Post) FillGrpcMessage(message *proto.Post) {
	message.Id.Value = post.Id
	message.AuthorId.Value = post.UserId
	message.Body = post.Body
	message.CreatedAt = post.CreatedAt
}

func PostFromGrpcMessage(message *proto.Post) Post {
	return Post{
		Id:        message.Id.Value,
		UserId:    message.AuthorId.Value,
		Body:      message.Body,
		CreatedAt: message.CreatedAt,
	}
}
