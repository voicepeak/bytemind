package agent

import "bytemind/internal/session"

type EventType string

const (
	EventRunStarted        EventType = "run_started"
	EventAssistantDelta    EventType = "assistant_delta"
	EventAssistantMessage  EventType = "assistant_message"
	EventToolCallStarted   EventType = "tool_call_started"
	EventToolCallCompleted EventType = "tool_call_completed"
	EventPlanUpdated       EventType = "plan_updated"
	EventRunFinished       EventType = "run_finished"
)

type Event struct {
	Type          EventType
	SessionID     string
	UserInput     string
	Content       string
	ToolName      string
	ToolArguments string
	ToolResult    string
	Error         string
	Plan          []session.PlanItem
}

type Observer interface {
	HandleEvent(Event)
}

type ObserverFunc func(Event)

func (f ObserverFunc) HandleEvent(event Event) {
	f(event)
}
