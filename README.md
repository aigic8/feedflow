# FeedFlow

This is an Go app given a list of RSS feeds, it will notify when a new feed is found in any of them.

## TO DO

- [ ] handle deleting feeds and notify users
- [ ] add docker-compose file
- [ ] add docs for Kubernetes
- [ ] refactor to a modular code
- [ ] add tests
- [ ] add CI/CD pipeline for building a docker image and pushing to image repository and testing
- [ ] support for images in the feeds
- [ ] support for when a feed is not supported (doesn't have an specific field)
  - user should be notified and feed should be marked in the db
