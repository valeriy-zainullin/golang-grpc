package models

import (
	proto "main/gen"

	"gorm.io/gorm"
)

type User struct {
	Id       uint64 `gorm:"primaryKey"`
	Nickname string
	PhotoUrl string

	Posts []Post
	Likes []Like
}

func (user *User) Create(db *gorm.DB) error {
	return db.Create(user).Error
}

func (user *User) Save(db *gorm.DB) error {
	return db.Save(user).Error
}

func (user *User) Delete(db *gorm.DB) error {
	return db.Delete(user).Error
}

func FindUserById(db *gorm.DB, id uint64) (User, error) {
	var user User
	result := db.First(&user, id)
	return user, result.Error
}

func (user *User) FillGrpcMessage(message *proto.User) {
	message.Id = &proto.UserId{Value: user.Id}
	message.Nickname = user.Nickname
	message.PhotoUrl = &proto.Url{Href: user.PhotoUrl}
}

func UserFromGrpcMessage(message *proto.User) User {
	return User{
		Id:       message.Id.Value,
		Nickname: message.Nickname,
		PhotoUrl: message.PhotoUrl.Href,
	}
}
