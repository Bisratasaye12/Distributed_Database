package security

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

// AllUser represents the permissions any user has in the auth store.
const AllUser = "*"

// Creds is used to parse credential entries from JSON.
type Creds struct {
	Username    string   `json:"username,omitempty"`
	Password    string   `json:"password,omitempty"`
	Permissions []string `json:"perms,omitempty"`
}

// CredentialsStore handles authentication and permission management.
type CredentialsStore struct {
	passwords   map[string][]byte
	permissions map[string]map[string]struct{}
}

// NewAuthStore returns an initialized CredentialsStore.
func NewAuthStore() CredentialsStore {
	return CredentialsStore{
		passwords:   make(map[string][]byte),
		permissions: make(map[string]map[string]struct{}),
	}
}

// Initialize reads JSON from an io.Reader and populates the AuthStore.
func (cs *CredentialsStore) Initialize(r io.Reader) error {
	decoder := json.NewDecoder(r)
	if _, err := decoder.Token(); err != nil {
		return err
	}

	var cred Creds
	for decoder.More() {
		if err := decoder.Decode(&cred); err != nil {
			return err
		}

		// Hash password before storing it
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(cred.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Error hashing password for user %s: %v", cred.Username, err)
			return err
		}
		cs.passwords[cred.Username] = hashedPassword

		// Initialize and populate permissions map
		cs.permissions[cred.Username] = make(map[string]struct{}, len(cred.Permissions))
		for _, perm := range cred.Permissions {
			cs.permissions[cred.Username][perm] = struct{}{}
		}
	}

	if _, err := decoder.Token(); err != nil {
		return err
	}

	return nil
}

// SetForAllUser sets given permissions for every user.
func (cs *CredentialsStore) SetForAllUser(perms ...string) {
	if cs.permissions[AllUser] == nil {
		cs.permissions[AllUser] = make(map[string]struct{})
	}
	for _, perm := range perms {
		cs.permissions[AllUser][perm] = struct{}{}
	}
}

// ValidatePassword checks a hashed password against the stored hash.
func (cs *CredentialsStore) ValidatePassword(username string, password []byte) bool {
	pass, ok := cs.passwords[username]
	if !ok {
		return false
	}
	return bcrypt.CompareHashAndPassword(pass, password) == nil
}

// CheckReq reads the auth information from the request header and checks the password.
func (cs *CredentialsStore) CheckReq(r *http.Request) bool {
	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return cs.ValidatePassword(username, []byte(password))
}

// HasPermission checks if a user has the given permission.
func (cs *CredentialsStore) HasPermission(username, permission string) bool {
	if perms, ok := cs.permissions[username]; ok {
		if _, exists := perms[permission]; exists {
			return true
		}
	}
	return false
}

// HasPermissionReq checks the HTTP request for the given permission.
func (cs *CredentialsStore) HasPermissionReq(r *http.Request, perm string) bool {
	username, _, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return cs.HasPermission(username, perm)
}
