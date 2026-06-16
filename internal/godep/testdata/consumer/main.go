package main

import (
	"fmt"

	"git.acme.local/platform/auth"
	"github.com/spf13/viper"
)

func main() {
	c := auth.NewClient()
	fmt.Println(c.Verify("x"))
	_ = viper.New()
}
