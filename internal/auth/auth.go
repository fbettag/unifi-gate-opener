package auth

import (
	"net/http"

	"github.com/gorilla/sessions"
)

const (
	SessionName = "gate-opener-session"
	UserKey     = "authenticated"
)

type SessionStore struct {
	store *sessions.CookieStore
}

func NewSessionStore(secret string) *SessionStore {
	return &SessionStore{
		store: sessions.NewCookieStore([]byte(secret)),
	}
}

func (s *SessionStore) GetSession(r *http.Request) (*sessions.Session, error) {
	session, err := s.store.Get(r, SessionName)
	if err != nil {
		// If session is corrupted, create a new one
		session, _ = s.store.New(r, SessionName)
	}

	// Set session options
	session.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	}

	return session, nil
}

func (s *SessionStore) SaveSession(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	return session.Save(r, w)
}

func (s *SessionStore) IsAuthenticated(r *http.Request) bool {
	session, err := s.GetSession(r)
	if err != nil {
		return false
	}

	auth, ok := session.Values[UserKey].(bool)
	return ok && auth
}

func (s *SessionStore) Login(r *http.Request, w http.ResponseWriter) error {
	session, err := s.GetSession(r)
	if err != nil {
		return err
	}

	session.Values[UserKey] = true
	return s.SaveSession(r, w, session)
}

func (s *SessionStore) Logout(r *http.Request, w http.ResponseWriter) error {
	session, err := s.GetSession(r)
	if err != nil {
		return err
	}

	// Clear session values
	session.Values = make(map[interface{}]interface{})
	session.Options.MaxAge = -1

	return s.SaveSession(r, w, session)
}
