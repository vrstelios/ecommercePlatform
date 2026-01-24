package database

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"os"
)

var Conn *pgx.Conn

func InitDatabase() {

	var err error
	cnnDB := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Tehran",
		os.Getenv("HOST"), os.Getenv("PORT_DB"), os.Getenv("USER"), os.Getenv("PASSWORD"), os.Getenv("NAME"), os.Getenv("SSLMODE"))

	Conn, err = pgx.Connect(context.Background(), cnnDB)
	if err != nil {
		panic("Failed to connect to db: " + err.Error())
	}
}
