package main

import (
	"os"
)

func main() {
	os.Exit(1) // want "direct call to os.Exit is forbidden"

	defer func() {
		os.Exit(2) // want "direct call to os.Exit is forbidden"
	}()

	// Безопасные вызовы функций пакета os не должны помечаться.
	os.Setenv("FOO", "bar")
}

func helper() {
	os.Exit(3)
}
