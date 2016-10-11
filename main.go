// Exercise 4.11: Build a tool that lets users create, read, update, and
// delete GitHub issues from the command line, invoking their preferred
// text editor when substantial text input is required.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	apiURL       = "https://api.github.com"
	acceptHeader = "application/vnd.github.v3+json"
	authToken    = "token ba0c7dd43b263ef005f41c3fda7e6fc7e76394bd"
)

var action string
var editor bool
var owner string
var repos string
var id int
var title string
var body string

func init() {
	const (
		defaultAction = "read"
		defaultEditor = false
	)

	const (
		shorthand   = " (shorthand)"
		usageAction = "Create, read or update"
		usageEditor = "Use preferred editor to create or update issue body"
		usageOwner  = "Repository owner eg. gwhn"
		usageRepos  = "Repository name eg. go-play"
		usageID     = "ID of issue"
		usageTitle  = "Title of issue"
		usageBody   = "Body of issue"
	)

	flag.StringVar(&action, "action", defaultAction, usageAction)
	flag.StringVar(&action, "a", defaultAction, usageAction+shorthand)

	flag.BoolVar(&editor, "editor", defaultEditor, usageEditor)
	flag.BoolVar(&editor, "e", defaultEditor, usageEditor+shorthand)

	flag.StringVar(&owner, "owner", "", usageOwner)
	flag.StringVar(&owner, "o", "", usageOwner+shorthand)

	flag.StringVar(&repos, "repos", "", usageRepos)
	flag.StringVar(&repos, "r", "", usageRepos+shorthand)

	flag.IntVar(&id, "id", 0, usageID)
	flag.IntVar(&id, "i", 0, usageID+shorthand)

	flag.StringVar(&title, "title", "", usageTitle)
	flag.StringVar(&title, "t", "", usageTitle+shorthand)

	flag.StringVar(&body, "body", "", usageBody)
	flag.StringVar(&body, "b", "", usageBody+shorthand)
}

// ghiss	-a[ction]=create
//				-e[ditor]
//				-o[wner]=gwhn
//				-r[epos]=go-play
//				-t[itle]="the title"
//				-b[ody]="the body"
//
// ghiss	-a[ction]=read
//				-o[wner]=gwhn
//				-r[epos]=go-play
//				-i[d]=123
//
// ghiss	-a[ction]=update
//				-e[ditor]
//				-o[wner]=gwhn
//				-r[epos]=go-play
//				-i[d]=123
//				-t[itle]="updated title"
//				-b[ody]="updated body"

func main() {
	flag.Parse()

	switch strings.ToLower(action) {
	case "create":
		issue, err := create(owner, repos, title, body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating issue %q: %v\n", title, err)
			os.Exit(2)
		}
		fmt.Println("Created issue successfully")
		report(issue)
	case "update":
		issue, err := update(owner, repos, id, title, body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error update issue %d: %v\n", id, err)
			os.Exit(2)
		}
		fmt.Println("Updated issue successfully")
		report(issue)
	case "read":
		issue, err := read(owner, repos, id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading issue %d: %v\n", id, err)
			os.Exit(2)
		}
		report(issue)
	default: // should show error and exit?
		fmt.Fprintf(os.Stderr, "Unknown action %s\n", action)
		os.Exit(2)
	}
}

type RequestIssue struct {
	Title string `json:"title"`
	Body string `json:"body"`
}

func create(owner, repos, title, body string) (*Issue, error) {
	if len(owner) == 0 {
		return nil, fmt.Errorf("Owner is required to create issues")
	}
	if len(repos) == 0 {
		return nil, fmt.Errorf("Repository is required to create issues")
	}
	if len(title) == 0 {
		return nil, fmt.Errorf("Title is required to create issues")
	}

	// POST /repos/:owner/:repo/issues
	params := fmt.Sprintf("/repos/%s/%s/issues", owner, repos)
	url := fmt.Sprintf("%s%s", apiURL, params)
	data, err := json.Marshal(RequestIssue{
		Title: title,
		Body: body,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal to json: %v", err)
	}
	return request("POST", url, bytes.NewBuffer(data), http.StatusCreated)
}

func request(method, url string, reader io.Reader, status int) (*Issue, error) {
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", acceptHeader)
	req.Header.Set("Authorization", authToken)
	resp, err := http.DefaultClient.Do(req)
	if resp.StatusCode != status {
		resp.Body.Close()
		return nil, fmt.Errorf("Request %s %s failed: %s", method, url, resp.Status)
	}
	var result Issue
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		resp.Body.Close()
		return nil, err
	}
	resp.Body.Close()
	return &result, nil
}

func update(owner, repos string, id int, title, body string) (*Issue, error) {
	return nil, fmt.Errorf("Not implemented update")
}

type Issue struct {
	Number    int
	HTMLURL   string `json:"html_url"`
	Title     string
	State     string
	User      *User
	CreatedAt time.Time `json:"created_at"`
	Body      string    // in Markdown format
}

type User struct {
	Login string
}

func read(owner, repos string, id int) (*Issue, error) {
	if len(owner) == 0 {
		return nil, fmt.Errorf("Owner is required to read issues")
	}
	if len(repos) == 0 {
		return nil, fmt.Errorf("Repository is required to read issues")
	}
	if id < 1 {
		return nil, fmt.Errorf("Valid ID is required to read issues")
	}

	params := fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repos, id)
	url := fmt.Sprintf("%s%s", apiURL, params)
	return request("GET", url, nil, http.StatusOK)
}

func report(i *Issue) {
	hr := strings.Repeat("-", 70)
	fmt.Printf("%.70s\n", hr)
	fmt.Printf("%15.15s %d\n", "number:", i.Number)
	fmt.Printf("%15.15s %s\n", "url:", i.HTMLURL)
	fmt.Printf("%15.15s %s\n", "title:", i.Title)
	fmt.Printf("%15.15s %s\n", "state:", i.State)
	fmt.Printf("%15.15s %s\n", "user:", i.User.Login)
	fmt.Printf("%15.15s %s\n", "created at:", i.CreatedAt)
	fmt.Printf("\n%s\n", i.Body)
	fmt.Printf("%.70s\n", hr)
}
