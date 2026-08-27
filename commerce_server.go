package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sync"
)

type authProvider interface {
	VerifyCaptcha(ctx context.Context, widgetRecordID, token, ip string) error
	CreateUser(ctx context.Context, email, password, name, requestID string) (string, error)
}

type account struct {
	UserID       string
	Email        string
	PasswordHash [32]byte
}

type commerceServer struct {
	auth     authProvider
	pipeline *OrderPipeline
	mu       sync.Mutex
	accounts map[string]account
	sessions map[string]string
}

func newCommerceServer(auth authProvider) *commerceServer {
	return &commerceServer{auth: auth, pipeline: NewOrderPipeline(), accounts: make(map[string]account), sessions: make(map[string]string)}
}

func (s *commerceServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /signup", s.signup)
	mux.HandleFunc("POST /login", s.login)
	mux.HandleFunc("POST /checkout", s.requireSession(s.checkout))
	mux.HandleFunc("POST /orders/{id}/fulfill", s.requireSession(s.fulfill))
	return mux
}

func (s *commerceServer) signup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email          string `json:"email"`
		Password       string `json:"password"`
		Name           string `json:"name"`
		WidgetRecordID string `json:"widget_record_id"`
		Captcha        string `json:"captcha_token"`
		RequestID      string `json:"request_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if err := s.auth.VerifyCaptcha(r.Context(), in.WidgetRecordID, in.Captcha, host); err != nil {
		s.writeAuthError(w, err)
		return
	}
	userID, err := s.auth.CreateUser(r.Context(), in.Email, in.Password, in.Name, in.RequestID)
	if err != nil {
		s.writeAuthError(w, err)
		return
	}
	s.mu.Lock()
	s.accounts[in.Email] = account{UserID: userID, Email: in.Email, PasswordHash: sha256.Sum256([]byte(in.Password))}
	s.mu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]string{"user_id": userID})
}

func (s *commerceServer) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	s.mu.Lock()
	acct, ok := s.accounts[in.Email]
	s.mu.Unlock()
	want := sha256.Sum256([]byte(in.Password))
	if !ok || subtle.ConstantTimeCompare(acct.PasswordHash[:], want[:]) != 1 {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	sessionID, err := randomID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create session"})
		return
	}
	s.mu.Lock()
	s.sessions[sessionID] = acct.Email
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "commerce_session", Value: sessionID, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 3600})
	writeJSON(w, http.StatusOK, map[string]string{"user_id": acct.UserID})
}

func (s *commerceServer) checkout(w http.ResponseWriter, r *http.Request, email string) {
	var order Order
	if !decode(w, r, &order) {
		return
	}
	order.Email = email
	created, err := s.pipeline.Checkout(order)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *commerceServer) fulfill(w http.ResponseWriter, r *http.Request, _ string) {
	receipt, update, err := s.pipeline.Fulfill(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Receipt Receipt        `json:"receipt"`
		Update  CustomerUpdate `json:"customer_update"`
	}{receipt, update})
}

func (s *commerceServer) requireSession(next func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("commerce_session")
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "login required"})
			return
		}
		s.mu.Lock()
		email, ok := s.sessions[cookie.Value]
		s.mu.Unlock()
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "login required"})
			return
		}
		next(w, r, email)
	}
}

func (s *commerceServer) writeAuthError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	var apiErr *InfraiError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		status = apiErr.HTTPStatus
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func randomID() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
