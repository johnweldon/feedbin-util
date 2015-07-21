package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"
)

var (
	username string
	password string
	baseurl  string
	dryrun   bool
)

func init() {
	flag.StringVar(&username, "username", "", "feedbin.com username")
	flag.StringVar(&password, "password", "", "feedbin.com password")
	flag.StringVar(&baseurl, "baseurl", "https://api.feedbin.com/v2/", "feedbin.com base url for API")
	flag.BoolVar(&dryrun, "d", false, "dry run")
}

func main() {
	flag.Parse()
	cred := DefaultCredentials()
	err := RemoveBrokenSubscriptions(cred, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error removing broken subscriptions: %v\n", err)
		os.Exit(1)
	}
}

type Credentials struct {
	Username string
	Password string
	BaseURL  string
	DryRun   bool
}

func DefaultCredentials() Credentials {
	return Credentials{
		Username: username,
		Password: password,
		BaseURL:  baseurl,
		DryRun:   dryrun,
	}
}

type Subscription struct {
	ID      int       `json:"id,omitempty"`
	Created time.Time `json:"created_at,omitempty"`
	FeedID  int       `json:"feed_id,omitempty"`
	Title   string    `json:"title,omitempty"`
	FeedURL string    `json:"feed_url,omitempty"`
	SiteURL string    `json:"site_url,omitempty"`
}

type Subscriptions []Subscription

func RemoveBrokenSubscriptions(cred Credentials, log io.Writer) error {
	subscriptions, err := GetSubscriptions(cred)
	if err != nil {
		return err
	}

	for _, sub := range subscriptions {
		resp, err := http.Get(sub.FeedURL)
		if err != nil {
			fmt.Fprintf(log, "Could not GET %q; Error: %v - REMOVING\n", sub.FeedURL, err)
			err = RemoveSubscription(cred, sub)
			if err != nil {
				fmt.Fprintf(log, "Failed to REMOVE %q; Error: %v\n", sub.FeedURL, err)
			}
			continue
		}
		if resp == nil {
			fmt.Fprintf(log, " Error: %q returned nil response\n", sub.FeedURL)
			continue
		}
		switch resp.StatusCode {
		case http.StatusNotFound, http.StatusNotAcceptable, http.StatusForbidden, http.StatusUnauthorized:
			fmt.Fprintf(log, "Could not GET %q; Response: %s\n", sub.FeedURL, resp.Status)
			err = RemoveSubscription(cred, sub)
			if err != nil {
				fmt.Fprintf(log, "Failed to REMOVE %q; Error: %v\n", sub.FeedURL, err)
			}
		case http.StatusOK, http.StatusAccepted, http.StatusPartialContent:
		default:
			fmt.Fprintf(log, " Warning: %q returned %d\n", sub.FeedURL, resp.StatusCode)
		}
	}
	return nil
}

func GetSubscriptions(cred Credentials) (Subscriptions, error) {
	getURL, err := url.Parse(fmt.Sprintf("%s/subscriptions.json", cred.BaseURL))
	if err != nil {
		return Subscriptions{}, err
	}
	req, err := http.NewRequest("GET", getURL.String(), nil)
	if err != nil {
		return Subscriptions{}, err
	}
	req.SetBasicAuth(cred.Username, cred.Password)

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return Subscriptions{}, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return Subscriptions{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return Subscriptions{}, fmt.Errorf("Error: %s\n%s", resp.Status, data)
	}

	var s Subscriptions
	err = json.Unmarshal(data, &s)
	if err != nil {
		return s, err
	}

	return s, nil
}

func RemoveSubscription(cred Credentials, sub Subscription) error {
	if cred.DryRun {
		return nil
	}

	delURL, err := url.Parse(fmt.Sprintf("%s/subscriptions/%d.json", cred.BaseURL, sub.ID))
	if err != nil {
		return err
	}
	req, err := http.NewRequest("DELETE", delURL.String(), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(cred.Username, cred.Password)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Error: %s\n%s", resp.Status, body)
	}
	return nil
}
