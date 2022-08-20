// Package userdb implements a very simple, read-only user database.
package userdb

import (
	"encoding/json"
	"os"

	"golang.org/x/crypto/bcrypt"
)

type Authenticator interface {
	Authenticate(username, password string) error
}

type userdb map[string]string // username: bcrypt hash

func (db userdb) Authenticate(username, password string) error {
	storedHash, ok := db[username]
	if !ok {
		return bcrypt.ErrMismatchedHashAndPassword
	}
	return bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
}

func Open(path string) (Authenticator, error) {
	var db = userdb{}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		return db, json.Unmarshal(data, &db)
	case os.IsNotExist(err):
		return db, os.WriteFile(path, []byte("{}"), 0660)
	default:
		return nil, err
	}
}
