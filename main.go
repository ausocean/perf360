/*
NAME
  Perf360 is a simple web app for collecting 360-degree performance reviews.

AUTHOR
  Alan Noble <alan@ausocean.org>

LICENSE
  Copyright (c) 2026, The Perf360 Authors

  BSD 3-Clause License

  Redistribution and use in source and binary forms, with or without
  modification, are permitted provided that the following conditions are met:

  1. Redistributions of source code must retain the above copyright notice, this
     list of conditions and the following disclaimer.

  2. Redistributions in binary form must reproduce the above copyright notice,
     this list of conditions and the following disclaimer in the documentation
     and/or other materials provided with the distribution.

  3. Neither the name of The Australian Ocean Lab Ltd. ("AusOcean")
     nor the names of its contributors may be used to endorse or promote
     products derived from this software without specific prior written permission.

  THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
  AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
  IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
  DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
  FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
  DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
  SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
  CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
  OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
  OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

// Perf360 is a simple web app for collecting 360-degree performance reviews.
// Perf360 is configured via five environment variables:
//
//   - PERF360_SECRETS contains the OAuth2 client secret (clientSecret)
//     and the OAuth2 cookie session key (sessionKey).
//   - PREF360_USERS contains in JSON format the list users. See the User struct
//   - PERF360_CREDENTIALS contains the datastore client credentials.
//   - PERF360_PERIOD is the review period (typically a year).
//   - PERF360_STATUS is either open (when editing peer reviews) or closed (when reviewing peer reviews).

package main

import (
	"context"
	"errors"
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"sync"

	"github.com/ausocean/cloud/backend"
	"github.com/ausocean/cloud/datastore"
	"github.com/ausocean/cloud/gauth"
)

// pagedata defines the page template data.
type pagedata struct {
	Profile   *gauth.Profile
	LoginURL  string
	LogoutURL string
	Period    string
	Status    string
	User      *User
	Users     []User
	Review    *Review
	Reviews   []Review
	Member    string
	Footer    template.HTML
}

const (
	oauthClientID = "862497151247-qgmag7p2b9r7el7vri1gjl32ne5vbocm.apps.googleusercontent.com"
	oauthMaxAge   = 60 * 60 * 24 * 7 // 7 days
	footer        = "<footer><p>&copy;2024-2026 The Perf360 Authors</p></footer>"
)

var (
	projectID     = "perf360"
	setupMutex    sync.Mutex
	templates     = template.Must(template.New("").Funcs(templateFuncs).ParseGlob("t/*.html"))
	templateFuncs = template.FuncMap{}
	auth          *gauth.UserAuth
	dstore        datastore.Store
	users         []User
	debug         bool
	reviewPeriod  string = os.Getenv("PERF360_PERIOD")
	status        string = os.Getenv("PERF360_STATUS")
)

var (
	errMissingUser  = errors.New("missing user")
	errMissingPeer  = errors.New("missing peer")
	errInvalidEmail = errors.New("invalid email address")
	errUnauthorized = errors.New("unauthorized")
)

func main() {
	defaultPort := 8080
	v := os.Getenv("PORT")
	if v != "" {
		i, err := strconv.Atoi(v)
		if err == nil {
			defaultPort = i
		}
	}

	var port int
	var period string
	flag.BoolVar(&debug, "debug", false, "Run in debug mode.")
	flag.IntVar(&port, "port", defaultPort, "Port we listen on")
	flag.StringVar(&period, "period", "", "Review period")
	flag.Parse()
	if debug {
		log.Printf("running in debug mode")
	}
	if period != "" {
		reviewPeriod = period
	}
	if reviewPeriod == "" {
		log.Fatalf("reviewPeriod not defined")
	}
	log.Printf("review period: %s", reviewPeriod)
	if status != "open" && status != "closed" {
		log.Fatalf("invalid status: %q", status)
	}
	log.Printf("status: %s", status)

	// Static file handler (only when running locally).
	http.Handle("/s/", http.StripPrefix("/s/", http.FileServer(http.Dir("s"))))

	// OAuth2 handlers.
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/oauth2callback", oauthCallbackHandler)

	// App logic handlers.
	http.HandleFunc("/select", selectHandler)
	http.HandleFunc("/feedback", feedbackHandler)
	http.HandleFunc("/manage", manageHandler)
	http.HandleFunc("/", indexHandler)

	log.Printf("initializing OAuth2")
	auth = &gauth.UserAuth{ProjectID: projectID, ClientID: oauthClientID, MaxAge: oauthMaxAge}
	auth.Init(backend.NewNetHandler(nil, nil, nil))

	setup(context.Background())

	log.Printf("listening on %d", port)
	http.ListenAndServe(":"+strconv.Itoa(port), nil)
}

// setup sets up the datastore and users.
func setup(ctx context.Context) {
	setupMutex.Lock()
	defer setupMutex.Unlock()

	if dstore != nil {
		return
	}

	var err error
	dstore, err = datastore.NewStore(ctx, "cloud", projectID, "")
	if err != nil {
		log.Fatalf("error creating datastore: %v", err)
	}

	datastore.RegisterEntity(typeReview, func() datastore.Entity { return new(Review) })
	log.Printf("datastore ready")

	users, err = importUsers(ctx)
	if err != nil {
		log.Fatalf("error importing users: %v", err)
	}
	log.Printf("imported %d users", len(users))
}

// indexHandler is the main handler. If a peer has previously been
// selected, then the corresponding review is fetched. If not, an
// empty review is presented for the user to edit.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	if r.URL.Path != "/" {
		// Redirect all invalid URLs to the root homepage.
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	profile, err := getProfile(w, r)
	data := pagedata{
		Profile: profile,
		Users:   users,
		Review:  &Review{},
	}

	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		writeTemplate(w, r, "index.html", &data, "")
		return
	}

	user := findUser(profile.Email)
	if user == nil {
		writeError(w, errMissingUser, http.StatusBadRequest)
		return
	}
	data.User = user
	data.Users = removeUser(profile.Email)
	peer := profile.Data
	data.Review = &Review{Name: user.Name, Peer: peer}

	ctx := r.Context()
	setup(ctx)

	if status == "closed" {
		data.Reviews, err = getPeerReviews(ctx, dstore, user.Name)
		if err != nil {
			log.Printf("datastore error getting peer reviews for %s: %v", user.Name, err)
		}
	}

	if peer != "" {
		review, err := getReview(ctx, dstore, user.Name, peer)
		switch err {
		case nil:
			data.Review = review
		case datastore.ErrNoSuchEntity:
			// No review yet.
		default:
			log.Printf("datastore error getting review: %v", err)
		}
	}

	writeTemplate(w, r, "index.html", &data, "")
}

// selectHandler selects a peer to review.
func selectHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	profile, err := getProfile(w, r)

	data := pagedata{
		Profile: profile,
		Users:   users,
		Review:  &Review{},
	}
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		writeTemplate(w, r, "index.html", &data, "")
		return
	}

	peer := r.FormValue("peer") // OK to be empty. It just clears the page.
	putProfileData(w, r, peer)

	http.Redirect(w, r, "/", http.StatusFound)
}

// feedbackHandler saves peer feedback,
func feedbackHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	profile, err := getProfile(w, r)

	data := pagedata{
		Profile: profile,
		Users:   users,
		Review:  &Review{},
	}
	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		writeTemplate(w, r, "index.html", &data, "")
		return
	}

	user := findUser(profile.Email)
	if user == nil {
		writeError(w, errMissingUser, http.StatusBadRequest)
		return
	}
	peer := profile.Data
	if peer == "" {
		writeError(w, errMissingPeer, http.StatusBadRequest)
		return
	}

	feedback := r.FormValue("feedback")

	// Save peer feedback, which may be empty initially.
	ctx := r.Context()
	setup(ctx)
	review := &Review{Name: user.Name, Peer: peer, Feedback: feedback}
	err = putReview(ctx, dstore, review)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

// manageHandler handles manager requests.
func manageHandler(w http.ResponseWriter, r *http.Request) {
	logRequest(r)

	profile, err := getProfile(w, r)
	data := pagedata{
		Profile: profile,
	}

	if err != nil {
		if err != gauth.TokenNotFound {
			log.Printf("authentication error: %v", err)
		}
		writeTemplate(w, r, "manage.html", &data, "")
		return
	}

	user := findUser(profile.Email)
	if user == nil {
		writeError(w, errMissingUser, http.StatusBadRequest)
		return
	}
	if !user.IsManager {
		writeError(w, errUnauthorized, http.StatusUnauthorized)
		return
	}

	if user.IsCEO {
		data.Users = users
	} else {
		data.Users = members(user.Name)
	}

	data.Member = r.FormValue("member")
	ctx := r.Context()
	setup(ctx)
	data.Reviews, err = getPeerReviews(ctx, dstore, data.Member)
	if err != nil {
		log.Printf("datastore error getting peer reviews for %s: %v", data.Member, err)
	}

	log.Printf("manager %s reviewing %s", user.Name, data.Member)
	writeTemplate(w, r, "manage.html", &data, "")
}

// findUser finds the user with the given email address.
func findUser(email string) *User {
	for _, user := range users {
		if user.Email == email {
			return &user
		}
	}
	return nil
}

// removeUser returns a copy of user list with the given email address removed.
func removeUser(email string) []User {
	var list []User
	for _, user := range users {
		if user.Email != email {
			list = append(list, user)
		}
	}
	return list
}

// members returns the users who are members of the manager's team.
func members(manager string) []User {
	var list []User
	for _, user := range users {
		if user.Manager == manager {
			list = append(list, user)
		}
	}
	return list
}

// writeTemplate renders a template.
func writeTemplate(w http.ResponseWriter, r *http.Request, name string, data interface{}, msg string) {
	v := reflect.Indirect(reflect.ValueOf(data))
	p := v.FieldByName("Profile")
	if p.IsValid() {
		profile, _ := getProfile(w, r)
		p.Set(reflect.ValueOf(profile))
	}
	p = v.FieldByName("LoginURL")
	if p.IsValid() {
		p.Set(reflect.ValueOf("/login?redirect=" + r.URL.RequestURI()))
	}
	p = v.FieldByName("LogoutURL")
	if p.IsValid() {
		p.Set(reflect.ValueOf("/logout?redirect=" + r.URL.RequestURI()))
	}
	p = v.FieldByName("Period")
	if p.IsValid() {
		p.Set(reflect.ValueOf(reviewPeriod))
	}
	p = v.FieldByName("Status")
	if p.IsValid() {
		p.Set(reflect.ValueOf(status))
	}
	p = v.FieldByName("Footer")
	if p.IsValid() {
		p.Set(reflect.ValueOf(template.HTML(footer)))
	}

	err := templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Fatalf("ExecuteTemplate failed on %s: %v", name, err)
	}
}

// loginHandler handles user login requests.
func loginHandler(w http.ResponseWriter, r *http.Request) {
	err := auth.LoginHandler(backend.NewNetHandler(w, r, auth.NetStore))
	if err != nil {
		writeError(w, err, http.StatusBadRequest)
	}
}

// logoutHandler handles user logout requests.
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	err := auth.LogoutHandler(backend.NewNetHandler(w, r, auth.NetStore))
	if err != nil {
		writeError(w, err, http.StatusBadRequest)
	}
}

// oauthCallbackHandler implements the OAuth2 callback that completes the authentication process.
func oauthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	_, err := auth.CallbackHandler(backend.NewNetHandler(w, r, auth.NetStore))
	if err != nil {
		writeError(w, err, http.StatusBadRequest)
	}
}

// getProfile returns the profile for the logged-in user.
func getProfile(w http.ResponseWriter, r *http.Request) (*gauth.Profile, error) {
	return auth.GetProfile(backend.NewNetHandler(w, r, auth.NetStore))
}

// putProfileData puts profile data.
func putProfileData(w http.ResponseWriter, r *http.Request, val string) error {
	return auth.PutData(backend.NewNetHandler(w, r, auth.NetStore), val)
}

// writeError returns an HTTP error. It is currently just a wrapper for http.Error.
func writeError(w http.ResponseWriter, err error, code int) {
	http.Error(w, err.Error(), code)
}

// logRequest logs a requet in debug mode.
func logRequest(r *http.Request) {
	if debug {
		return
	}
	if r.URL.RawQuery == "" {
		log.Println(r.URL.Path)
		return
	}
	log.Println(r.URL.Path + "?" + r.URL.RawQuery)
}
