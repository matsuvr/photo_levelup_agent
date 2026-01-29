package handlers

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/session"
)

type Dependencies struct {
	Agent          agent.Agent
	SessionService session.Service
}

func NewDependencies(agent agent.Agent, sessionService session.Service) *Dependencies {
	return &Dependencies{
		Agent:          agent,
		SessionService: sessionService,
	}
}
