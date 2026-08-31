package main

import (
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

func composePublisher(session *project.Session) *publisher.Publisher {
	return publisher.New(session, project.Version)
}

func preparePublisher(composed *publisher.Publisher) (publisher.Preparation, error) {
	return composed.Prepare()
}

func operationPreparation(session *project.Session) (publisher.Preparation, error) {
	return preparePublisher(composePublisher(session))
}
