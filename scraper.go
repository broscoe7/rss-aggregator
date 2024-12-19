package main

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/broscoe7/rss-aggregator/internal/database"
	"github.com/google/uuid"
)

func startScraping(
	db *database.Queries,
	concurrency int,
	timeBetweenRequests time.Duration,
) {
	log.Printf("Scraping on %v goroutines every %s duration", concurrency, timeBetweenRequests)
	ticker := time.NewTicker(timeBetweenRequests)
	// This way the loop runs immediately before waiting on the ticker
	// The loop will repeat after every time interval specified by the ticker
	for ; ; <-ticker.C {
		feeds, err := db.GetNextFeedsToFetch(context.Background(), int32(concurrency))
		if err != nil {
			log.Println("error fetching feeds: ", err)
			continue
		}

		wg := &sync.WaitGroup{}
		for _, feed := range feeds {
			wg.Add(1)
			go scrapeFeed(feed, db, wg)
		}
		wg.Wait()
	}
}

func scrapeFeed(feed database.Feed, db *database.Queries, wg *sync.WaitGroup) {
	defer wg.Done()
	_, err := db.MarkFeedAsFetched(context.Background(), feed.ID)
	if err != nil {
		log.Println("Error marking feed as fetched: ", err)
		return
	}
	rssFeed, err := urlToFeed(feed.Url)
	if err != nil {
		log.Println("Error fetching feed: ", err)
		return
	}

	for _, item := range rssFeed.Channel.Item {
		// Prepare description since it is nullable
		description := sql.NullString{}
		if item.Description != "" {
			description.String = item.Description
			description.Valid = true
		}
		// Parse the published_at timestamp
		pubAt, err := time.Parse(time.RFC1123Z, item.PubDate)
		if err != nil {
			log.Printf("Couldn't parse date %v with error %v", item.PubDate, err)
			continue
		}
		_, err = db.CreatePost(context.Background(), database.CreatePostParams{
			ID: uuid.New(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
			Title: item.Title,
			Description: description,
			PublishedAt: pubAt,
			Url: item.Link,
			FeedID: feed.ID,
		})
		if err != nil {
			// If there is a duplicate post, we do not save to the DB and quietly skip over the error
			if strings.Contains(err.Error(), "duplicate key") {
				continue
			}
			log.Println("Failed to create post: ", err)
		}
	}
	log.Printf("Feed %s collected; %v posts found", feed.Name, len(rssFeed.Channel.Item))
}