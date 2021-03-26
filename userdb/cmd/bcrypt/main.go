package main

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh/terminal"
)

func main() {

	fmt.Print("new password: ")

	password, err := terminal.ReadPassword(0)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(string(hash))
}
