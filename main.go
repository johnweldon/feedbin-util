package main

import (
	"encoding/json"
	"errors"
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
	BaseUrl  string
	DryRun   bool
}

func DefaultCredentials() Credentials {
	return Credentials{
		Username: username,
		Password: password,
		BaseUrl:  baseurl,
		DryRun:   dryrun,
	}
}

type Subscription struct {
	Id      int       `json:"id,omitempty"`
	Created time.Time `json:"created_at,omitempty"`
	FeedId  int       `json:"feed_id,omitempty"`
	Title   string    `json:"title,omitempty"`
	FeedUrl string    `json:"feed_url,omitempty"`
	SiteUrl string    `json:"site_url,omitempty"`
}

type Subscriptions []Subscription

func RemoveBrokenSubscriptions(cred Credentials, log io.Writer) error {
	subscriptions, err := GetSubscriptions(cred)
	if err != nil {
		return err
	}

	for _, sub := range subscriptions {
		resp, err := http.Get(sub.FeedUrl)
		if err != nil {
			fmt.Fprintf(log, "Could not GET %q; Error: %v - REMOVING\n", sub.FeedUrl, err)
			err = RemoveSubscription(cred, sub)
			if err != nil {
				fmt.Fprintf(log, "Failed to REMOVE %q; Error: %v\n", sub.FeedUrl, err)
			}
			continue
		}
		if resp == nil {
			fmt.Fprintf(log, " Error: %q returned nil response\n", sub.FeedUrl)
			continue
		}
		switch resp.StatusCode {
		case http.StatusNotFound, http.StatusNotAcceptable, http.StatusForbidden, http.StatusUnauthorized:
			fmt.Fprintf(log, "Could not GET %q; Response: %s\n", sub.FeedUrl, resp.Status)
			err = RemoveSubscription(cred, sub)
			if err != nil {
				fmt.Fprintf(log, "Failed to REMOVE %q; Error: %v\n", sub.FeedUrl, err)
			}
		case http.StatusOK, http.StatusAccepted, http.StatusPartialContent:
		default:
			fmt.Fprintf(log, " Warning: %q returned %d\n", sub.FeedUrl, resp.StatusCode)
		}
	}
	return nil
}

func GetSubscriptions(cred Credentials) (Subscriptions, error) {
	getUrl, err := url.Parse(fmt.Sprintf("%s/subscriptions.json", cred.BaseUrl))
	if err != nil {
		return Subscriptions{}, err
	}
	req, err := http.NewRequest("GET", getUrl.String(), nil)
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
		return Subscriptions{}, errors.New(fmt.Sprintf("Error: %s\n%s", resp.Status, data))
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

	delUrl, err := url.Parse(fmt.Sprintf("%s/subscriptions/%d.json", cred.BaseUrl, sub.Id))
	if err != nil {
		return err
	}
	req, err := http.NewRequest("DELETE", delUrl.String(), nil)
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
		return errors.New(fmt.Sprintf("Error: %s\n%s", resp.Status, body))
	}
	return nil
}
