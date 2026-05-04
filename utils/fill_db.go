package utils

import (
	"log"
	"main/models"

	"gorm.io/gorm"
)

func FillDB(db *gorm.DB) {
	users := []models.User{
		{Id: 1, Nickname: "Admin", PhotoUrl: "https://avatars.mds.yandex.net/get-shedevrum/13699184/img_94a88488490511ef8752aeb7c6f335f1/orig"},
	}

	// Save creates the row, if it does not exist yet.

	for _, user := range users {
		err := user.Save(db)

		if err != nil {
			log.Fatalf("failed to create or update a user: %v", user)
		}
	}

	posts := []models.Post{
		{
			Id:     1,
			UserId: 1,
			Body:   "Hello, world! This is the first post of this blog.",
		},
	}

	for _, post := range posts {
		err := post.Save(db)

		if err != nil {
			log.Fatalf("failed to create or update a post: %v", post)
		}
	}
}
