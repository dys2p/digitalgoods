package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/dys2p/digitalgoods/db"
	"github.com/dys2p/digitalgoods/html"
)

type HandlerErrFunc func(http.ResponseWriter, *http.Request) error

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

func wrapTmpl(f HandlerErrFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := f(w, r); err != nil {
			var msg string
			if db.IsNotFound(err) {
				msg = html.GetLanguage(r).Translate("error-purchase-not-found")
			} else {
				log.Printf("error: %v", err)
				msg = html.GetLanguage(r).Translate("error-internal") + err.Error()
			}
			html.Error.Execute(w, msg)
		}
	}
}
