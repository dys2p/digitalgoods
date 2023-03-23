package main

import (
	"errors"
	"log"
	"net/http"
	gopath "path"

	"github.com/dys2p/digitalgoods/db"
	"github.com/dys2p/digitalgoods/html"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/text/language"
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
			var msg string
			if db.IsNotFound(err) {
				msg = html.Language("en").Translate("error-purchase-not-found")
			} else {
				log.Printf("error: %v", err)
				msg = html.Language("en").Translate("error-internal") + err.Error()
			}
			html.Error.Execute(w, msg)
		}
	}
}

// TODO inline into handlers, or test at startup
func wrapLangTmpl(f HandlerLangErrFunc) func(w http.ResponseWriter, r *http.Request, lang string) {
	return func(w http.ResponseWriter, r *http.Request, lang string) {
		if err := f(w, r, lang); err != nil {
			var msg string
			if db.IsNotFound(err) {
				msg = html.Language(lang).Translate("error-purchase-not-found")
			} else {
				log.Printf("error: %v", err)
				msg = html.Language(lang).Translate("error-internal") + err.Error()
			}
			html.Error.Execute(w, msg)
		}
	}
}

func addRoutes(router *httprouter.Router, langs []string, method, path string, handler func(w http.ResponseWriter, r *http.Request, lang string)) {
	// register one handler for each language
	for _, lang := range langs {
		lang := lang
		router.HandlerFunc(
			method,
			gopath.Join("/", lang, path),
			func(w http.ResponseWriter, r *http.Request) {
				handler(w, r, lang)
			},
		)
	}
	// prepare matcher
	tags := make([]language.Tag, len(langs))
	for i := range langs {
		tags[i] = language.Make(langs[i])
	}
	matcher := language.NewMatcher(tags)
	// redirect
	if len(langs) > 0 {
		router.HandlerFunc(
			method,
			path,
			func(w http.ResponseWriter, r *http.Request) {
				_, i := language.MatchStrings(matcher, r.Header.Get("Accept-Language"))
				http.Redirect(w, r, gopath.Join("/", langs[i], r.URL.Path), http.StatusSeeOther)
			},
		)
	}
}
