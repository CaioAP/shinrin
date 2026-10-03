package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/CaioAP/shinrin/backend/internal/domain"
	"github.com/CaioAP/shinrin/backend/internal/port"
)

// The Nuxt server keeps the session token in an HttpOnly cookie and passes
// it here as "Authorization: Bearer <token>". The browser never sees the Go
// API, so the cookie and CSRF checks live in Nuxt.

type authHandler struct {
	svc port.AccountService
}

// userHandler is a handler that needs a signed-in user.
type userHandler func(w http.ResponseWriter, r *http.Request, u domain.User)

// requireUser resolves the bearer token to a user or answers 401.
func (h authHandler) requireUser(next userHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := h.svc.Authenticate(r.Context(), bearerToken(r))
		if err != nil {
			writeDomainError(w, err)
			return
		}
		next(w, r, u)
	}
}

func bearerToken(r *http.Request) string {
	v := r.Header.Get("Authorization")
	if t, ok := strings.CutPrefix(v, "Bearer "); ok {
		return strings.TrimSpace(t)
	}
	return ""
}

// maxBody bounds JSON request bodies.
const maxBody = 64 << 10

// decodeJSON reads a JSON body into v, rejecting unknown fields.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("%w: request body is empty", domain.ErrInvalid)
		}
		return fmt.Errorf("%w: request body is not valid JSON", domain.ErrInvalid)
	}
	return nil
}

type credentialsBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// signUp handles POST /api/v1/auth/signup.
func (h authHandler) signUp(w http.ResponseWriter, r *http.Request) {
	var b credentialsBody
	if err := decodeJSON(w, r, &b); err != nil {
		writeDomainError(w, err)
		return
	}
	u, tok, err := h.svc.SignUp(r.Context(), b.Email, b.Password)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sessionDTO{User: toUserDTO(u), Token: tok.Value, ExpiresAt: tok.ExpiresAt.UTC().Format(timeFormat)})
}

// signIn handles POST /api/v1/auth/signin.
func (h authHandler) signIn(w http.ResponseWriter, r *http.Request) {
	var b credentialsBody
	if err := decodeJSON(w, r, &b); err != nil {
		writeDomainError(w, err)
		return
	}
	u, tok, err := h.svc.SignIn(r.Context(), b.Email, b.Password)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionDTO{User: toUserDTO(u), Token: tok.Value, ExpiresAt: tok.ExpiresAt.UTC().Format(timeFormat)})
}

// signOut handles POST /api/v1/auth/signout. It succeeds even when the
// session is already gone.
func (h authHandler) signOut(w http.ResponseWriter, r *http.Request) {
	if tok := bearerToken(r); tok != "" {
		if err := h.svc.SignOut(r.Context(), tok); err != nil {
			writeDomainError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// me handles GET /api/v1/me.
func (h authHandler) me(w http.ResponseWriter, _ *http.Request, u domain.User) {
	writeJSON(w, http.StatusOK, toUserDTO(u))
}

// setRiskProfile handles PUT /api/v1/me/risk-profile.
func (h authHandler) setRiskProfile(w http.ResponseWriter, r *http.Request, u domain.User) {
	var b struct {
		Answers map[string]string `json:"answers"`
	}
	if err := decodeJSON(w, r, &b); err != nil {
		writeDomainError(w, err)
		return
	}
	updated, err := h.svc.SetRiskProfile(r.Context(), u.ID, b.Answers)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserDTO(updated))
}

// deleteAccount handles DELETE /api/v1/me with the password in the body.
func (h authHandler) deleteAccount(w http.ResponseWriter, r *http.Request, u domain.User) {
	var b struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &b); err != nil {
		writeDomainError(w, err)
		return
	}
	if err := h.svc.DeleteAccount(r.Context(), u.ID, b.Password); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// questionnaire handles GET /api/v1/risk-questionnaire: the question and
// answer codes; the web app owns the wording.
func questionnaire(w http.ResponseWriter, _ *http.Request) {
	out := make([]questionDTO, len(domain.SuitabilityQuestions))
	for i, q := range domain.SuitabilityQuestions {
		out[i] = questionDTO{ID: q.ID, Answers: q.Answers}
	}
	writeJSON(w, http.StatusOK, listDTO[questionDTO]{Items: out})
}
