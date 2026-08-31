package main

import (
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/publisher"
)

func composePublisher(session *project.Session) *publisher.Publisher {
	return publisher.New(session, project.Version)
}
