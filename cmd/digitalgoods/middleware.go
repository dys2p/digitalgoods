package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/dys2p/digitalgoods/db"
	"github.com/dys2p/digitalgoods/html"
)

type HandlerErrFunc func(http.ResponseWriter, *http.Request) error

type HandlerLangErrFunc func(http.ResponseWriter, *http.Request, string) error

var ErrUnauthenticated = errors.New("unauthenticated")

func auth(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if staffSessions.Exists(r.Context(), "username") {
			f(w, r)
		} else {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		}
	}
}

func wrapAPI(f HandlerErrFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		}
	}
}

// TODO inline into handlers, or test at startup
func wrapTmpl(f HandlerErrFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			if db.IsNotFound(err) {
				html.ErrorNotFound.Execute(w, nil)
			} else {
				html.ErrorInternal.Execute(w, nil)
				log.Printf("[internal server error] %v", err)
			}
		}
	}
}
