// Package userdb implements a very simple, read-only user database.
package userdb

import (
	"encoding/json"
	"os"

	"golang.org/x/crypto/bcrypt"
)

const Path = "data/users.json"

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

func Open() (Authenticator, error) {
	var db = userdb{}
	data, err := os.ReadFile(Path)
	switch {
	case err == nil:
		return db, json.Unmarshal(data, &db)
	case os.IsNotExist(err):
		return db, os.WriteFile(Path, []byte("{}"), 0660)
	default:
		return nil, err
	}
}
