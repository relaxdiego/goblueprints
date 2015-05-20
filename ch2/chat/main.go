package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"text/template"

	"github.com/stretchr/gomniauth"
	"github.com/stretchr/gomniauth/providers/google"
	"github.com/stretchr/objx"
)

// templ represents a single template
type templateHandler struct {
	once sync.Once

	filename string

	templ *template.Template
}

// ServeHTTP handles the HTTP request
func (t *templateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.once.Do(func() {
		t.templ = template.Must(template.ParseFiles(filepath.Join("templates", t.filename)))
	})
	data := map[string]interface{}{
		"Host": r.Host,
	}
	if authCookie, err := r.Cookie("auth"); err == nil {
		data["UserData"] = objx.MustFromBase64(authCookie.Value)
	}
	if err := t.templ.Execute(w, data); err != nil {
		log.Println("ERROR: Failed to render template", t.filename, "-", err)
	}
}

type appConfig struct {
	SecurityKey string
	Google      appProviderConfig
}

type appProviderConfig struct {
	ClientSecret string
	ClientID     string
}

func unmarshalConfig(path string) appConfig {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}

	var config appConfig
	err = json.Unmarshal(file, &config)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(config.SecurityKey)
	return config
}

func main() {
	var addr = flag.String("addr", ":8080", "The addr of the application.")
	flag.Parse() // parse the flags

	// load config.json
	config := unmarshalConfig(filepath.Join(".", "config.json"))

	// Set up gomniauth
	gomniauth.SetSecurityKey(config.SecurityKey)
	gomniauth.WithProviders(
		google.New(config.Google.ClientID, config.Google.ClientSecret, "http://localhost:8080/auth/callback/google"))

	// handle static files in /assets
	http.Handle("/assets/", http.StripPrefix("/assets", http.FileServer(http.Dir("assets"))))

	// chat page
	http.Handle("/chat", MustAuth(&templateHandler{filename: "chat.html"}))

	http.Handle("/login", &templateHandler{filename: "login.html"})

	http.HandleFunc("/auth/", loginHandler)

	// chatroom
	r := newRoom()
	// When /room is accessed, the ServeHTTP method of
	// the room will be called.
	http.Handle("/room", r)

	// get the room going
	go r.run()

	// start the web server
	log.Println("Starting web server on", *addr)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal("ListenAndServe", err)
	}
}
