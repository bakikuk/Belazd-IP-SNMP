package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bakikuk/belazd-ip-snmp/internal/config"
)

func (s *Services) handleUserGroups(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if _, ok := s.requirePerm(w, r, config.ModuleUserGroups, config.PermRead); !ok {
			return
		}
		s.mu.RLock()
		groups := append([]config.UserGroup(nil), s.cfg.UserGroups...)
		s.mu.RUnlock()
		writeJSON(w, groups)
	case http.MethodPost:
		if _, ok := s.requirePerm(w, r, config.ModuleUserGroups, config.PermWrite); !ok {
			return
		}
		var body struct {
			ID          string                   `json:"id"`
			Name        string                   `json:"name"`
			Permissions config.ModulePermissions `json:"permissions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		body.ID = strings.TrimSpace(body.ID)
		body.Name = strings.TrimSpace(body.Name)
		if err := config.ValidateGroupID(body.ID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if body.Name == "" {
			http.Error(w, "name required", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		for _, g := range s.cfg.UserGroups {
			if g.ID == body.ID {
				s.mu.Unlock()
				http.Error(w, "group id already exists", http.StatusConflict)
				return
			}
		}
		g := config.UserGroup{ID: body.ID, Name: body.Name, Permissions: body.Permissions}
		s.cfg.UserGroups = append(s.cfg.UserGroups, g)
		if err := s.cfg.ValidateUsersAndGroups(); err != nil {
			s.cfg.UserGroups = s.cfg.UserGroups[:len(s.cfg.UserGroups)-1]
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.persistUsersLocked(); err != nil {
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.mu.Unlock()
		writeJSON(w, g)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Services) handleUserGroupByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/user-groups/")
	id = strings.Trim(strings.TrimSuffix(id, "/"), " ")
	if id == "" {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		if _, ok := s.requirePerm(w, r, config.ModuleUserGroups, config.PermRead); !ok {
			return
		}
		s.mu.RLock()
		g := s.cfg.GroupByID(id)
		s.mu.RUnlock()
		if g == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		writeJSON(w, g)
	case http.MethodPatch:
		if _, ok := s.requirePerm(w, r, config.ModuleUserGroups, config.PermWrite); !ok {
			return
		}
		var body struct {
			Name        string                   `json:"name"`
			Permissions config.ModulePermissions `json:"permissions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		g := s.cfg.GroupByID(id)
		if g == nil {
			s.mu.Unlock()
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		old := *g
		if strings.TrimSpace(body.Name) != "" {
			g.Name = strings.TrimSpace(body.Name)
		}
		g.Permissions = body.Permissions
		if err := s.cfg.ValidateUsersAndGroups(); err != nil {
			*g = old
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if s.cfg.CountPrivilegedUsers() == 0 {
			*g = old
			s.mu.Unlock()
			http.Error(w, "cannot remove last full admin access from all users", http.StatusBadRequest)
			return
		}
		if err := s.persistUsersLocked(); err != nil {
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		out := *g
		s.mu.Unlock()
		writeJSON(w, out)
	case http.MethodDelete:
		if _, ok := s.requirePerm(w, r, config.ModuleUserGroups, config.PermDelete); !ok {
			return
		}
		s.mu.Lock()
		if s.cfg.UsersInGroup(id) > 0 {
			s.mu.Unlock()
			http.Error(w, "group still has users", http.StatusBadRequest)
			return
		}
		found := false
		for i, g := range s.cfg.UserGroups {
			if g.ID == id {
				s.cfg.UserGroups = append(s.cfg.UserGroups[:i], s.cfg.UserGroups[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			s.mu.Unlock()
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err := s.cfg.ValidateUsersAndGroups(); err != nil {
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.persistUsersLocked(); err != nil {
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Services) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if _, ok := s.requirePerm(w, r, config.ModuleUsers, config.PermRead); !ok {
			return
		}
		s.mu.RLock()
		type row struct {
			Username    string `json:"username"`
			GroupID     string `json:"groupId"`
			PasswordSet bool   `json:"passwordSet"`
		}
		var out []row
		for _, u := range s.cfg.Users {
			out = append(out, row{
				Username:    u.Username,
				GroupID:     u.GroupID,
				PasswordSet: strings.TrimSpace(u.PasswordHash) != "",
			})
		}
		s.mu.RUnlock()
		writeJSON(w, out)
	case http.MethodPost:
		if _, ok := s.requirePerm(w, r, config.ModuleUsers, config.PermWrite); !ok {
			return
		}
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
			GroupID  string `json:"groupId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		body.Username = strings.TrimSpace(body.Username)
		body.GroupID = strings.TrimSpace(body.GroupID)
		if err := config.ValidateUsername(body.Username); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := config.ValidatePasswordPolicy(body.Password); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		hash, err := config.HashPassword(body.Password)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.mu.Lock()
		for _, u := range s.cfg.Users {
			if u.Username == body.Username {
				s.mu.Unlock()
				http.Error(w, "username already exists", http.StatusConflict)
				return
			}
		}
		if s.cfg.GroupByID(body.GroupID) == nil {
			s.mu.Unlock()
			http.Error(w, "unknown group", http.StatusBadRequest)
			return
		}
		s.cfg.Users = append(s.cfg.Users, config.UserAccount{
			Username:     body.Username,
			GroupID:      body.GroupID,
			PasswordHash: hash,
		})
		if err := s.cfg.ValidateUsersAndGroups(); err != nil {
			s.cfg.Users = s.cfg.Users[:len(s.cfg.Users)-1]
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := s.persistUsersLocked(); err != nil {
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.mu.Unlock()
		writeJSON(w, map[string]interface{}{
			"username":    body.Username,
			"groupId":     body.GroupID,
			"passwordSet": true,
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Services) handleUserByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/users/")
	name = strings.Trim(strings.TrimSuffix(name, "/"), " ")
	if name == "" {
		http.Error(w, "bad username", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		if _, ok := s.requirePerm(w, r, config.ModuleUsers, config.PermRead); !ok {
			return
		}
		s.mu.RLock()
		var found *config.UserAccount
		for i := range s.cfg.Users {
			if s.cfg.Users[i].Username == name {
				found = &s.cfg.Users[i]
				break
			}
		}
		s.mu.RUnlock()
		if found == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]interface{}{
			"username":    found.Username,
			"groupId":     found.GroupID,
			"passwordSet": strings.TrimSpace(found.PasswordHash) != "",
		})
	case http.MethodPatch:
		if _, ok := s.requirePerm(w, r, config.ModuleUsers, config.PermWrite); !ok {
			return
		}
		var body struct {
			GroupID  string `json:"groupId"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		idx := -1
		for i := range s.cfg.Users {
			if s.cfg.Users[i].Username == name {
				idx = i
				break
			}
		}
		if idx < 0 {
			s.mu.Unlock()
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		old := s.cfg.Users[idx]
		if gid := strings.TrimSpace(body.GroupID); gid != "" {
			if s.cfg.GroupByID(gid) == nil {
				s.mu.Unlock()
				http.Error(w, "unknown group", http.StatusBadRequest)
				return
			}
			s.cfg.Users[idx].GroupID = gid
		}
		if body.Password != "" {
			if err := config.ValidatePasswordPolicy(body.Password); err != nil {
				s.mu.Unlock()
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			hash, err := config.HashPassword(body.Password)
			if err != nil {
				s.mu.Unlock()
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			s.cfg.Users[idx].PasswordHash = hash
		}
		if err := s.cfg.ValidateUsersAndGroups(); err != nil {
			s.cfg.Users[idx] = old
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if s.cfg.CountPrivilegedUsers() == 0 {
			s.cfg.Users[idx] = old
			s.mu.Unlock()
			http.Error(w, "cannot remove last full admin user", http.StatusBadRequest)
			return
		}
		if err := s.persistUsersLocked(); err != nil {
			s.cfg.Users[idx] = old
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		u := s.cfg.Users[idx]
		s.mu.Unlock()
		writeJSON(w, map[string]interface{}{
			"username":    u.Username,
			"groupId":     u.GroupID,
			"passwordSet": true,
		})
	case http.MethodDelete:
		if _, ok := s.requirePerm(w, r, config.ModuleUsers, config.PermDelete); !ok {
			return
		}
		s.mu.Lock()
		idx := -1
		for i := range s.cfg.Users {
			if s.cfg.Users[i].Username == name {
				idx = i
				break
			}
		}
		if idx < 0 {
			s.mu.Unlock()
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		removed := s.cfg.Users[idx]
		s.cfg.Users = append(s.cfg.Users[:idx], s.cfg.Users[idx+1:]...)
		if err := s.cfg.ValidateUsersAndGroups(); err != nil {
			s.cfg.Users = append(s.cfg.Users, removed)
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if len(s.cfg.Users) > 0 && s.cfg.CountPrivilegedUsers() == 0 {
			s.cfg.Users = append(s.cfg.Users, removed)
			s.mu.Unlock()
			http.Error(w, "cannot delete last full admin user", http.StatusBadRequest)
			return
		}
		if err := s.persistUsersLocked(); err != nil {
			s.cfg.Users = append(s.cfg.Users, removed)
			s.mu.Unlock()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Services) handleMePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.auth == nil || !s.auth.Enabled() {
		http.Error(w, "login disabled", http.StatusBadRequest)
		return
	}
	user, ok := s.sessionUser(r)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": "unauthorized", "loginRequired": true})
		return
	}
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.NewPassword != body.ConfirmPassword {
		http.Error(w, "password confirmation does not match", http.StatusBadRequest)
		return
	}
	if err := config.ValidatePasswordPolicy(body.NewPassword); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !s.auth.checkCredentials(user, body.CurrentPassword) {
		http.Error(w, "current password incorrect", http.StatusUnauthorized)
		return
	}
	hash, err := config.HashPassword(body.NewPassword)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	idx := -1
	for i := range s.cfg.Users {
		if s.cfg.Users[i].Username == user {
			idx = i
			break
		}
	}
	if idx < 0 {
		s.mu.Unlock()
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	s.cfg.Users[idx].PasswordHash = hash
	if err := s.persistUsersLocked(); err != nil {
		s.mu.Unlock()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.mu.Unlock()
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *Services) persistUsersLocked() error {
	if err := s.db.SaveConfig(s.cfg); err != nil {
		return err
	}
	return s.syncHTTPAuth()
}
