package main

import "github.com/ideagate/server-proxy/app"

func main() {
	configFileName := "./config.yaml"
	app.NewServer(configFileName)
}
