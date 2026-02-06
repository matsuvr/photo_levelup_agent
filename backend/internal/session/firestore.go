// Package session provides Firestore-based session management for ADK.
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"log"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/adk/session"
	"google.golang.org/api/iterator"
	"google.golang.org/genai"
)

const (
	sessionsCollection = "sessions"
	eventsCollection   = "events"
)

// FirestoreSession implements session.Session interface.
type FirestoreSession struct {
	id             string
	appName        string
	userID         string
	state          *FirestoreState
	events         *FirestoreEvents
	lastUpdateTime time.Time
}

func (s *FirestoreSession) ID() string                { return s.id }
func (s *FirestoreSession) AppName() string           { return s.appName }
func (s *FirestoreSession) UserID() string            { return s.userID }
func (s *FirestoreSession) State() session.State      { return s.state }
func (s *FirestoreSession) Events() session.Events    { return s.events }
func (s *FirestoreSession) LastUpdateTime() time.Time { return s.lastUpdateTime }

// FirestoreState implements session.State interface.
type FirestoreState struct {
	mu   sync.RWMutex
	data map[string]any
}

func NewFirestoreState(data map[string]any) *FirestoreState {
	if data == nil {
		data = make(map[string]any)
	}
	return &FirestoreState{data: data}
}

func (s *FirestoreState) Get(key string) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if val, ok := s.data[key]; ok {
		return val, nil
	}
	return nil, fmt.Errorf("key '%s' not found in session state: %w", key, session.ErrStateKeyNotExist)
}

func (s *FirestoreState) Set(key string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return nil
}

func (s *FirestoreState) All() iter.Seq2[string, any] {
	return func(yield func(string, any) bool) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		for k, v := range s.data {
			if !yield(k, v) {
				return
			}
		}
	}
}

func (s *FirestoreState) ToMap() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]any, len(s.data))
	for k, v := range s.data {
		result[k] = v
	}
	return result
}

// FirestoreEvents implements session.Events interface.
type FirestoreEvents struct {
	events []*session.Event
}

func NewFirestoreEvents(events []*session.Event) *FirestoreEvents {
	if events == nil {
		events = []*session.Event{}
	}
	return &FirestoreEvents{events: events}
}

func (e *FirestoreEvents) All() iter.Seq[*session.Event] {
	return func(yield func(*session.Event) bool) {
		for _, ev := range e.events {
			if !yield(ev) {
				return
			}
		}
	}
}

func (e *FirestoreEvents) Len() int {
	return len(e.events)
}

func (e *FirestoreEvents) At(i int) *session.Event {
	if i < 0 || i >= len(e.events) {
		return nil
	}
	return e.events[i]
}

// SessionDocument represents a session stored in Firestore.
type SessionDocument struct {
	ID        string         `firestore:"id"`
	AppName   string         `firestore:"appName"`
	UserID    string         `firestore:"userId"`
	State     map[string]any `firestore:"state"`
	CreatedAt time.Time      `firestore:"createdAt"`
	UpdatedAt time.Time      `firestore:"updatedAt"`
}

// EventDocument represents an event stored in Firestore.
type EventDocument struct {
	ID           string    `firestore:"id"`
	InvocationID string    `firestore:"invocationId"`
	Author       string    `firestore:"author"`
	Branch       string    `firestore:"branch"`
	Content      any       `firestore:"content"`
	Timestamp    time.Time `firestore:"timestamp"`
}

// FirestoreService implements session.Service using Cloud Firestore.
type FirestoreService struct {
	client    *firestore.Client
	projectID string
}

// NewFirestoreService creates a new Firestore-based session service.
func NewFirestoreService(ctx context.Context, projectID string) (*FirestoreService, error) {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Firestore client: %w", err)
	}

	return &FirestoreService{
		client:    client,
		projectID: projectID,
	}, nil
}

// Close closes the Firestore client.
func (s *FirestoreService) Close() error {
	return s.client.Close()
}

// sessionDocRef returns the document reference for a session.
func (s *FirestoreService) sessionDocRef(appName, userID, sessionID string) *firestore.DocumentRef {
	return s.client.Collection(sessionsCollection).Doc(fmt.Sprintf("%s_%s_%s", appName, userID, sessionID))
}

// Create creates a new session.
func (s *FirestoreService) Create(ctx context.Context, req *session.CreateRequest) (*session.CreateResponse, error) {
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("session_%d", time.Now().UnixNano())
	}

	now := time.Now()
	doc := SessionDocument{
		ID:        sessionID,
		AppName:   req.AppName,
		UserID:    req.UserID,
		State:     req.State,
		CreatedAt: now,
		UpdatedAt: now,
	}

	docRef := s.sessionDocRef(req.AppName, req.UserID, sessionID)
	_, err := docRef.Set(ctx, doc)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	log.Printf("Created session: %s for user: %s", sessionID, req.UserID)

	sess := &FirestoreSession{
		id:             sessionID,
		appName:        req.AppName,
		userID:         req.UserID,
		state:          NewFirestoreState(req.State),
		events:         NewFirestoreEvents(nil),
		lastUpdateTime: now,
	}

	return &session.CreateResponse{Session: sess}, nil
}

// Get retrieves a session by ID.
func (s *FirestoreService) Get(ctx context.Context, req *session.GetRequest) (*session.GetResponse, error) {
	docRef := s.sessionDocRef(req.AppName, req.UserID, req.SessionID)
	doc, err := docRef.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var sessionDoc SessionDocument
	if err := doc.DataTo(&sessionDoc); err != nil {
		return nil, fmt.Errorf("failed to decode session: %w", err)
	}

	// Get events
	events, err := s.getEvents(ctx, docRef)
	if err != nil {
		log.Printf("Warning: failed to get events: %v", err)
		events = NewFirestoreEvents(nil)
	}

	sess := &FirestoreSession{
		id:             sessionDoc.ID,
		appName:        sessionDoc.AppName,
		userID:         sessionDoc.UserID,
		state:          NewFirestoreState(sessionDoc.State),
		events:         events,
		lastUpdateTime: sessionDoc.UpdatedAt,
	}

	return &session.GetResponse{Session: sess}, nil
}

// List lists all sessions for a user.
func (s *FirestoreService) List(ctx context.Context, req *session.ListRequest) (*session.ListResponse, error) {
	query := s.client.Collection(sessionsCollection).
		Where("appName", "==", req.AppName).
		Where("userId", "==", req.UserID).
		OrderBy("updatedAt", firestore.Desc)

	iter := query.Documents(ctx)
	defer iter.Stop()

	var sessions []session.Session
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate sessions: %w", err)
		}

		var sessionDoc SessionDocument
		if err := doc.DataTo(&sessionDoc); err != nil {
			log.Printf("Warning: failed to decode session document: %v", err)
			continue
		}

		sess := &FirestoreSession{
			id:             sessionDoc.ID,
			appName:        sessionDoc.AppName,
			userID:         sessionDoc.UserID,
			state:          NewFirestoreState(sessionDoc.State),
			events:         NewFirestoreEvents(nil), // Don't load events for list
			lastUpdateTime: sessionDoc.UpdatedAt,
		}
		sessions = append(sessions, sess)
	}

	return &session.ListResponse{Sessions: sessions}, nil
}

// Delete deletes a session.
func (s *FirestoreService) Delete(ctx context.Context, req *session.DeleteRequest) error {
	docRef := s.sessionDocRef(req.AppName, req.UserID, req.SessionID)

	// Delete all events first
	eventsRef := docRef.Collection(eventsCollection)
	eventsIter := eventsRef.Documents(ctx)
	batch := s.client.Batch()
	for {
		doc, err := eventsIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			eventsIter.Stop()
			return fmt.Errorf("failed to iterate events for deletion: %w", err)
		}
		batch.Delete(doc.Ref)
	}
	eventsIter.Stop()

	// Commit event deletions
	if _, err := batch.Commit(ctx); err != nil {
		return fmt.Errorf("failed to delete events: %w", err)
	}

	// Delete session
	if _, err := docRef.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	log.Printf("Deleted session: %s", req.SessionID)
	return nil
}

// AppendEvent appends an event to a session.
func (s *FirestoreService) AppendEvent(ctx context.Context, sess session.Session, event *session.Event) error {
	docRef := s.sessionDocRef(sess.AppName(), sess.UserID(), sess.ID())

	// Log event details for debugging
	var contentPreview string
	if event.Content != nil {
		if len(event.Content.Parts) > 0 && event.Content.Parts[0].Text != "" {
			text := event.Content.Parts[0].Text
			if len(text) > 100 {
				contentPreview = text[:100] + "..."
			} else {
				contentPreview = text
			}
		}
	}
	log.Printf("INFO: AppendEvent called for session %s, author=%s, role=%s, content preview: %s",
		sess.ID(), event.Author, event.Content.Role, contentPreview)

	// Get current state from the session
	stateMap := make(map[string]any)
	for k, v := range sess.State().All() {
		stateMap[k] = v
	}

	// Update session timestamp and state
	_, err := docRef.Update(ctx, []firestore.Update{
		{Path: "updatedAt", Value: time.Now()},
		{Path: "state", Value: stateMap},
	})
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	// Serialize content to JSON for persistence
	var contentJSON string
	if event.Content != nil {
		if b, err := json.Marshal(event.Content); err == nil {
			contentJSON = string(b)
			log.Printf("INFO: Successfully serialized event content to JSON, length=%d bytes", len(contentJSON))
		} else {
			log.Printf("ERROR: Failed to marshal event content: %v", err)
		}
	} else {
		log.Printf("WARN: Event has nil Content, no JSON will be stored")
	}

	// Store event
	eventDoc := EventDocument{
		ID:           event.ID,
		InvocationID: event.InvocationID,
		Author:       event.Author,
		Branch:       event.Branch,
		Content:      contentJSON,
		Timestamp:    event.Timestamp,
	}

	_, _, err = docRef.Collection(eventsCollection).Add(ctx, eventDoc)
	if err != nil {
		log.Printf("ERROR: Failed to append event to Firestore for session %s: %v", sess.ID(), err)
		return fmt.Errorf("failed to append event: %w", err)
	}

	log.Printf("INFO: Successfully appended event to Firestore for session %s (author=%s)", sess.ID(), event.Author)

	// Update in-memory events so the ADK runner sees the new event
	// during the same invocation (ContentsRequestProcessor reads from session.Events()).
	if fSess, ok := sess.(*FirestoreSession); ok {
		fSess.events.events = append(fSess.events.events, event)
	}

	return nil
}

// getEvents retrieves all events for a session.
func (s *FirestoreService) getEvents(ctx context.Context, sessionDoc *firestore.DocumentRef) (*FirestoreEvents, error) {
	log.Printf("INFO: getEvents called for session document: %s", sessionDoc.Path)

	eventsIter := sessionDoc.Collection(eventsCollection).OrderBy("timestamp", firestore.Asc).Documents(ctx)
	defer eventsIter.Stop()

	var events []*session.Event
	retrievedCount := 0
	for {
		doc, err := eventsIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Printf("ERROR: Failed to iterate events for session %s: %v", sessionDoc.Path, err)
			return nil, fmt.Errorf("failed to iterate events: %w", err)
		}

		retrievedCount++
		var eventDoc EventDocument
		if err := doc.DataTo(&eventDoc); err != nil {
			log.Printf("WARN: Failed to decode event document %d for session %s: %v", retrievedCount, sessionDoc.Path, err)
			continue
		}

		event := session.NewEvent(eventDoc.InvocationID)
		event.ID = eventDoc.ID
		event.Author = eventDoc.Author
		event.Branch = eventDoc.Branch
		event.Timestamp = eventDoc.Timestamp

		// Restore content from stored JSON
		if contentStr, ok := eventDoc.Content.(string); ok && contentStr != "" {
			var content genai.Content
			if err := json.Unmarshal([]byte(contentStr), &content); err == nil {
				event.Content = &content
				log.Printf("INFO: Successfully restored event %d: author=%s, role=%s, timestamp=%v",
					retrievedCount, event.Author, content.Role, event.Timestamp)
			} else {
				log.Printf("WARN: Failed to unmarshal event content for event %d (session %s): %v", retrievedCount, sessionDoc.Path, err)
			}
		} else {
			log.Printf("WARN: Event %d has empty or non-string content (session %s)", retrievedCount, sessionDoc.Path)
		}

		events = append(events, event)
	}

	log.Printf("INFO: getEvents completed for session %s: retrieved %d events, returning %d valid events",
		sessionDoc.Path, retrievedCount, len(events))

	return NewFirestoreEvents(events), nil
}

// UpdateState updates the state of a session in Firestore.
// This method persists state changes directly to Firestore without requiring an AppendEvent call.
func (s *FirestoreService) UpdateState(ctx context.Context, appName, userID, sessionID string, updates map[string]any) error {
	docRef := s.sessionDocRef(appName, userID, sessionID)

	// Get current state
	doc, err := docRef.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get session for state update: %w", err)
	}

	var sessionDoc SessionDocument
	if err := doc.DataTo(&sessionDoc); err != nil {
		return fmt.Errorf("failed to decode session: %w", err)
	}

	// Merge updates into existing state
	if sessionDoc.State == nil {
		sessionDoc.State = make(map[string]any)
	}
	for key, value := range updates {
		sessionDoc.State[key] = value
	}

	// Update Firestore
	_, err = docRef.Update(ctx, []firestore.Update{
		{Path: "state", Value: sessionDoc.State},
		{Path: "updatedAt", Value: time.Now()},
	})
	if err != nil {
		return fmt.Errorf("failed to update session state: %w", err)
	}

	log.Printf("Updated session state for session %s: keys=%v", sessionID, keysFromMap(updates))
	return nil
}

// keysFromMap returns the keys of a map for logging purposes.
func keysFromMap(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Ensure FirestoreService implements session.Service
var _ session.Service = (*FirestoreService)(nil)
