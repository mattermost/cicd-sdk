package main

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

func main() {
	fmt.Println("This is just a test program that enables tests to ")
	fmt.Println("produce a binary to check it for reproductibility \n ")
	logrus.Info("Hello World!")
	logrus.Info("Replacement: %REPLACE_ME%")
}
