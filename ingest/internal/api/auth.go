package api

/*
Name: ingest/internal/api/auth.go
Description: Provides OAuth2-style authentication endpoints for the frontend dashboard.
Programmer: Adam Berry
Date Created: 2026-03-27
Dates Revised: 2026-03-27
Revision History:
- 2026-03-27, Adam: Added OAuth2-style token, revoke, and current-user handlers backed by a seeded test user.
Preconditions:
- Auth requests are sent to the versioned API base path.
Acceptable Input Values/Types:
- POST /oauth/token with password grant form data.
- GET /auth/me with a Bearer token issued by tokenHandler.
Unacceptable Input Values/Types:
- Malformed JSON bodies.
- Missing or invalid credentials or bearer tokens.
Postconditions:
- Successful token exchange returns an access token and username for frontend session storage.
Return Values/Types:
- Handlers write JSON responses directly.
Error/Exception Conditions:
- Invalid input returns 400 or 401.
Side Effects:
- Seeds the shared test user when the API has database access.
Invariants:
- The issued token always matches the configured username/password pair.
Known Faults:
- None
*/

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
)

const (
	defaultAuthUserID   = "test-user"
	defaultAuthUsername = "test"
	defaultAuthPassword = "test"
)

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
	Username    string `json:"username"`
}

type oauthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

type authMeResponse struct {
	Username string `json:"username"`
}

func (cfg *apiConfig) tokenHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		respondWithJSON(w, http.StatusBadRequest, oauthErrorResponse{
			Error:            "invalid_request",
			ErrorDescription: "invalid token request",
		})
		return
	}

	if r.FormValue("grant_type") != "password" {
		respondWithJSON(w, http.StatusBadRequest, oauthErrorResponse{
			Error:            "unsupported_grant_type",
			ErrorDescription: "grant_type must be password",
		})
		return
	}

	reqUsername := r.FormValue("username")
	reqPassword := r.FormValue("password")
	expectedUsername, expectedPassword := authCredentials()
	if subtle.ConstantTimeCompare([]byte(reqUsername), []byte(expectedUsername)) != 1 ||
		subtle.ConstantTimeCompare([]byte(reqPassword), []byte(expectedPassword)) != 1 {
		respondWithJSON(w, http.StatusUnauthorized, oauthErrorResponse{
			Error:            "invalid_grant",
			ErrorDescription: "invalid username or password",
		})
		return
	}

	respondWithJSON(w, http.StatusOK, oauthTokenResponse{
		AccessToken: issueAuthToken(expectedUsername, expectedPassword),
		TokenType:   "Bearer",
		ExpiresIn:   3600,
		Scope:       "dashboard",
		Username:    expectedUsername,
	})
}

func (cfg *apiConfig) revokeHandler(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "revoked"})
}

func (cfg *apiConfig) meHandler(w http.ResponseWriter, r *http.Request) {
	username, ok := authenticatedUsername(r)
	if !ok {
		respondWithJSON(w, http.StatusUnauthorized, oauthErrorResponse{
			Error:            "invalid_token",
			ErrorDescription: "unauthorized",
		})
		return
	}

	respondWithJSON(w, http.StatusOK, authMeResponse{Username: username})
}

func (cfg *apiConfig) authOptionsHandler(w http.ResponseWriter, _ *http.Request) {
	setAuthCORSHeaders(w)
	w.WriteHeader(http.StatusNoContent)
}

func authCredentials() (string, string) {
	return defaultAuthUsername, defaultAuthPassword
}

func issueAuthToken(username, password string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", username, password)))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func authenticatedUsername(r *http.Request) (string, bool) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", false
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	username, password := authCredentials()
	expectedToken := issueAuthToken(username, password)
	if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
		return "", false
	}

	return username, true
}

func setAuthCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
}

func ensureSeedAuthUser(queries *database.Queries) {
	if queries == nil {
		return
	}

	ctx := context.Background()
	_, err := queries.LookupUser(ctx, defaultAuthUsername)
	if err == nil {
		return
	}
	if err != nil && err != sql.ErrNoRows {
		log.Printf("Auth seed lookup failed: %v", err)
		return
	}

	_, err = queries.CreateUser(ctx, database.CreateUserParams{
		ID:       defaultAuthUserID,
		Username: defaultAuthUsername,
	})
	if err != nil {
		log.Printf("Auth seed create failed: %v", err)
	}
}
