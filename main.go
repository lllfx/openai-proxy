package main

import (
	"flag"
	"fmt"
	"github.com/lllfx/openai-proxy/api"

	"github.com/gin-gonic/gin"
)

func main() {
	// Define a flag for the port
	port := flag.Int("port", 8080, "Port to listen on")
	flag.Parse()

	// Create a new Gin router
	router := gin.Default()
	api.Register(router)

	// Run the server on port 8080
	err := router.Run(fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}
}
