package main

import "github.com/bayu-aditya/ideagate/backend/server/proxy/app"

func main() {
	configFileName := "./config.yaml"
	app.NewServer(configFileName)
}
