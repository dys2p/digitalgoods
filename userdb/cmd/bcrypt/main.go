package main

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func main() {

	fmt.Print("new password: ")

	password, err := term.ReadPassword(0)
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
