package main

import (
        "log"
        "os"

        "github.com/mrjones/oauth"
)

func main() {
        var consumerKey string = os.Args[1]
        var consumerSecret string = os.Args[2]

        var tweet string = string(os.Args[5])

        c := oauth.NewConsumer(
                consumerKey,
                consumerSecret,
                oauth.ServiceProvider{
                        RequestTokenUrl:   "http://api.twitter.com/oauth/request_token",
                        AuthorizeTokenUrl: "https://api.twitter.com/oauth/authorize",
                        AccessTokenUrl:    "https://api.twitter.com/oauth/access_token",
                })

        at := oauth.AccessToken{Token: os.Args[3], Secret: os.Args[4] }

        _, err := c.Post(
                "http://api.twitter.com/1/statuses/update.json",
                "",
                map[string]string{
                        "key": "",
                        "status": tweet,
                },
                &at)

        if err != nil {
                log.Fatal(err)
        }
}
