// SPDX-FileCopyrightText: 2021 Lightmeter <hello@lightmeter.io>
//
// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/sessions"
	"gitlab.com/lightmeter/controlcenter/auth"
	"gitlab.com/lightmeter/controlcenter/pkg/ctxlogger"
	"gitlab.com/lightmeter/controlcenter/pkg/httperror"
	"gitlab.com/lightmeter/controlcenter/util/errorutil"
	"gitlab.com/lightmeter/controlcenter/util/httputil"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"time"
)

type CookieStoreRegistrar struct {
	*auth.Auth
	workspaceDirectory string
}

const SessionDuration = time.Hour * 24 * 7 // 1 week

func (r *CookieStoreRegistrar) CookieStore() sessions.Store {
	sessionsDir := path.Join(r.workspaceDirectory, "http_sessions")
	errorutil.MustSucceed(os.MkdirAll(sessionsDir, os.ModePerm), "Creating http sessions directory")
	store := sessions.NewFilesystemStore(sessionsDir, r.Auth.SessionKeys()...)
	store.Options.HttpOnly = true
	store.Options.MaxAge = int(SessionDuration.Seconds())
	store.Options.SameSite = http.SameSiteStrictMode

	return store
}

func NewAuthenticator(auth *auth.Auth, workspaceDirectory string) *Authenticator {
	return NewAuthenticatorWithOptions(
		&CookieStoreRegistrar{
			Auth:               auth,
			workspaceDirectory: workspaceDirectory,
		},
	)
}

type SessionData struct {
	ID    int
	Email string
	Name  string
}

func (s SessionData) IsAuthenticated() bool {
	return len(s.Email) > 0
}

func init() {
	gob.Register(&SessionData{})
}

type Authenticator struct {
	Registrar auth.Registrar
	Store     sessions.Store
}

type RegistrarCookieStore interface {
	auth.Registrar
	CookieStore() sessions.Store
}

func NewAuthenticatorWithOptions(
	registrar RegistrarCookieStore,
) *Authenticator {
	return &Authenticator{registrar, registrar.CookieStore()}
}

const SessionName = "controlcenter"

func handleForm(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return fmt.Errorf("Error http method mismatch: %v", r.Method)
	}

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return fmt.Errorf("Error parse media type: %w", err)
	}

	if mediaType != "application/x-www-form-urlencoded" {
		return fmt.Errorf("Error media type mismatch: %w", err)
	}

	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("Error parse form: %w", err)
	}

	return nil
}

func HandleLogin(auth *Authenticator, w http.ResponseWriter, r *http.Request) error {
	if err := handleForm(w, r); err != nil {
		return httperror.NewHTTPStatusCodeError(http.StatusInternalServerError, errorutil.Wrap(err))
	}

	email := r.Form.Get("email")
	password := r.Form.Get("password")

	authOk, userData, err := auth.Registrar.Authenticate(r.Context(), email, password)

	if err != nil {
		return httperror.NewHTTPStatusCodeError(http.StatusInternalServerError, fmt.Errorf("authentication: %w", err))
	}

	if !authOk {
		return httperror.NewHttpCodeJsonError(http.StatusUnauthorized, errors.New("Invalid email address or password"))
	}

	session, err := auth.Store.New(r, SessionName)
	if err != nil {
		ctxlogger.LogErrorf(r.Context(), errorutil.Wrap(err), "creating new session")
	}

	session.Values["auth"] = SessionData{Email: email, Name: userData.Name, ID: userData.Id}

	if err := session.Save(r, w); err != nil {
		return httperror.NewHTTPStatusCodeError(http.StatusInternalServerError, fmt.Errorf("saving session on login: %w", err))
	}

	w.WriteHeader(http.StatusOK)

	return nil
}

type registrationResponse struct {
	Error    string
	Detailed interface{}
}

func handleRegistrationFailure(err error, w http.ResponseWriter, r *http.Request) error {
	response := registrationResponse{
		Error: errorutil.TryToUnwrap(err).Error(),

		Detailed: func() interface{} {
			if e, ok := errorutil.ErrorAs(err, &auth.PasswordValidationError{}); ok {
				//nolint:errorlint
				d, _ := e.(*auth.PasswordValidationError)
				return &d.Result
			}

			return nil
		}(),
	}

	if err := httputil.WriteJson(w, response, http.StatusUnauthorized); err != nil {
		return httperror.NewHTTPStatusCodeError(http.StatusInternalServerError, errorutil.Wrap(err))
	}

	return nil
}

func extractRegistrationFormInfo(r *http.Request) (string, string, string, error) {
	email, hasEmail := r.Form["email"]
	if !hasEmail || len(email) == 0 {
		return "", "", "", errors.New("email is missing")
	}

	password, hasPassword := r.Form["password"]
	if !hasPassword || len(password) == 0 {
		return "", "", "", errors.New("password is missing")
	}

	name, hasName := r.Form["name"]
	if !hasName || len(name) == 0 {
		return "", "", "", errors.New("name is missing")
	}

	return email[0], name[0], password[0], nil
}

func HandleRegistration(auth *Authenticator, w http.ResponseWriter, r *http.Request) error {
	if err := handleForm(w, r); err != nil {
		return httperror.NewHTTPStatusCodeError(http.StatusInternalServerError, errorutil.Wrap(err))
	}

	email, name, password, err := extractRegistrationFormInfo(r)

	if err != nil {
		return httperror.NewHTTPStatusCodeError(http.StatusBadRequest, fmt.Errorf("Registration: %w", err))
	}

	id, err := auth.Registrar.Register(r.Context(), email, name, password)
	if err != nil {
		return handleRegistrationFailure(err, w, r)
	}

	session, err := auth.Store.New(r, SessionName)
	if err != nil {
		ctxlogger.LogErrorf(r.Context(), errorutil.Wrap(err), "creating new session")
	}

	// Implicitly log in
	session.Values["auth"] = SessionData{Email: email, Name: name, ID: int(id)}
	if err := session.Save(r, w); err != nil {
		return httperror.NewHTTPStatusCodeError(http.StatusInternalServerError, fmt.Errorf("saving session on Login: %w", err))
	}

	w.WriteHeader(http.StatusOK)

	return nil
}

func HandleLogout(w http.ResponseWriter, r *http.Request, session *sessions.Session) error {
	sessionData, ok := session.Values["auth"].(*SessionData)
	if !(ok && sessionData.IsAuthenticated()) {
		return httperror.NewHTTPStatusCodeError(http.StatusUnauthorized, errors.New("unauthorized"))
	}

	log.Println("User", sessionData.Email, "logs out")

	session.Values["auth"] = nil

	if err := session.Save(r, w); err != nil {
		return httperror.NewHTTPStatusCodeError(http.StatusInternalServerError, fmt.Errorf("saving session on Login: %w", err))
	}

	return nil
}

// do not redirect to any page
func IsNotLoginOrNotRegistered(auth *Authenticator, w http.ResponseWriter, r *http.Request) error {
	hasAnyUser, err := auth.Registrar.HasAnyUser(r.Context())
	if err != nil {
		return httperror.NewHTTPStatusCodeError(http.StatusInternalServerError, errorutil.Wrap(fmt.Errorf("check has any users: %w", err)))
	}

	if !hasAnyUser {
		return httperror.NewHTTPStatusCodeError(http.StatusForbidden, errors.New("forbidden"))
	}

	session, err := auth.Store.Get(r, SessionName)
	if err != nil {
		return httperror.NewHTTPStatusCodeError(http.StatusUnauthorized, errors.New("unauthorized"))
	}

	sessionData, ok := session.Values["auth"].(*SessionData)
	if !(ok && sessionData.IsAuthenticated()) {
		return httperror.NewHTTPStatusCodeError(http.StatusUnauthorized, errors.New("unauthorized"))
	}

	w.WriteHeader(http.StatusOK)

	return nil
}

var ErrUnauthenticated = errors.New(`Unauthenticated`)

func GetSessionData(auth *Authenticator, r *http.Request) (*SessionData, error) {
	session, err := auth.Store.Get(r, SessionName)
	if err != nil {
		return nil, errorutil.Wrap(err)
	}

	sessionData, ok := session.Values["auth"].(*SessionData)
	if !(ok && sessionData.IsAuthenticated()) {
		return nil, ErrUnauthenticated
	}

	return sessionData, nil
}

func HandleGetUserData(auth *Authenticator, w http.ResponseWriter, r *http.Request) error {
	sessionData, err := GetSessionData(auth, r)
	if err != nil {
		return httperror.NewHTTPStatusCodeError(http.StatusUnauthorized, errors.New("unauthorized: is not authenticated"))
	}

	// refresh user data
	userData, err := auth.Registrar.GetUserDataByID(r.Context(), sessionData.ID)
	if err != nil {
		// FIXME: we should check for ErrInvalidUserID, implemented in the base "auth" package!
		if errors.Is(err, sql.ErrNoRows) {
			return httperror.NewHTTPStatusCodeError(http.StatusNotFound, fmt.Errorf("not found (id: %v)", sessionData.ID))
		}

		return httperror.NewHTTPStatusCodeError(http.StatusInternalServerError, errorutil.Wrap(err))
	}

	b, err := json.Marshal(userData)
	if err != nil {
		return httperror.NewHTTPStatusCodeError(http.StatusInternalServerError, errorutil.Wrap(err))
	}

	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(b); err != nil {
		return httperror.NewHTTPStatusCodeError(http.StatusInternalServerError, errorutil.Wrap(err))
	}

	return nil
}
