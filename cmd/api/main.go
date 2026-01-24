package main

import (
	"E-CommercePlatform/internal"
	"E-CommercePlatform/internal/database"
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"
)

func main() {
	srv := internal.New()

	fmt.Println(`
	 ______     ______         ______     ______   __
	/\  ___\   /\  __ \       /\  __ \   /\  == \ /\ \
	\ \ \__ \  \ \ \/\ \   -  \ \  __ \  \ \  _-/ \ \ \
	 \ \_____\  \ \_____\  -   \ \_\ \_\  \ \_\    \ \_\
	  \/_____/   \/_____/       \/_/\/_/   \/_/     \/_/ `)

	srv.Run(os.Getenv("ADDRESS") + ":" + os.Getenv("PORT"))
}

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	database.InitDatabase()
}
