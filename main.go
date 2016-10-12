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
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	apiURL       = "https://api.github.com"
	acceptHeader = "application/vnd.github.v3+json"
	authToken    = "token d15a404cdb7d819da98d9c41d2053ca89cd0057f"
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

	if editor {
		err := edit(id, &body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error editing issue body: %v\n", err)
			os.Exit(2)
		}
	}

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

func edit(id int, body *string) error {
	ed := os.Getenv("EDITOR")
	if len(ed) == 0 {
		return fmt.Errorf("$EDITOR environment variable not exported")
	}
	if id > 0 {
		issue, err := read(owner, repos, id)
		if err != nil {
			return fmt.Errorf("Failed to read issue %d: %v\n", id, err)
		}
		*body = issue.Body
	}
	tmpfile, err := write([]byte(*body))
	if err != nil {
		return fmt.Errorf("Failed to write temp file: %v\n", err)
	}
	defer os.Remove(tmpfile.Name())
	if err := open(tmpfile, ed); err != nil {
		return fmt.Errorf("Failed to open editor: %v\n", err)
	}
	bytes, err := ioutil.ReadFile(tmpfile.Name())
	if err != nil {
		return fmt.Errorf("Failed to read temp file: %v\n", err)
	}
	*body = string(bytes)
	return nil
}

func open(f *os.File, ed string) error {
	fmt.Println(ed, f.Name())
	cmd := exec.Command(ed, f.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func write(b []byte) (*os.File, error) {
	tmpfile, err := ioutil.TempFile("", "scratch")
	if err != nil {
		return nil, err
	}
	if _, err := tmpfile.Write(b); err != nil {
		return nil, err
	}
	if err := tmpfile.Close(); err != nil {
		return nil, err
	}
	return tmpfile, nil
}

type RequestIssue struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
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

	post := RequestIssue{}
	if len(title) > 0 {
		post.Title = title
	}
	if len(body) > 0 {
		post.Body = body
	}

	// POST /repos/:owner/:repo/issues
	resource := fmt.Sprintf("/repos/%s/%s/issues", owner, repos)
	url := fmt.Sprintf("%s%s", apiURL, resource)
	data, err := json.Marshal(post)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal to json: %v", err)
	}
	return request(http.MethodPost, url, bytes.NewBuffer(data), http.StatusCreated, true)
}

func request(method, url string, reader io.Reader, status int, auth bool) (*Issue, error) {
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", acceptHeader)
	if auth {
		req.Header.Set("Authorization", authToken)
	}
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
	if len(owner) == 0 {
		return nil, fmt.Errorf("Owner required to update issues")
	}
	if len(repos) == 0 {
		return nil, fmt.Errorf("Repository required to update issues")
	}
	if id < 1 {
		return nil, fmt.Errorf("Valid ID is required to update issues")
	}

	patch := RequestIssue{}
	if len(title) > 0 {
		patch.Title = title
	}
	if len(body) > 0 {
		patch.Body = body
	}
	
	// PATCH /repos/:owner/:repo/issues/:number
	resource := fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repos, id)
	url := fmt.Sprintf("%s%s", apiURL, resource)
	data, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal to json: %v", err)
	}
	return request(http.MethodPatch, url, bytes.NewBuffer(data), http.StatusOK, true)
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
	
	// GET /repos/:owner/:repo/issues/:number
	resource := fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repos, id)
	url := fmt.Sprintf("%s%s", apiURL, resource)
	return request(http.MethodGet, url, nil, http.StatusOK, false)
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
