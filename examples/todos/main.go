/*
 *    Copyright 2025 Jeff Galyan
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jrgalyan/quokka"
)

type Todo struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type store struct {
	mu    sync.RWMutex
	next  int64
	items map[int64]*Todo
}

func newStore() *store { return &store{next: 1, items: map[int64]*Todo{}} }

func (s *store) list(offset, limit int) ([]*Todo, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Gather sorted by id asc
	res := make([]*Todo, 0, len(s.items))
	for _, v := range s.items {
		res = append(res, v)
	}
	// No heavy sorting for simplicity; iteration order on map is random. Copy IDs and sort if needed.
	// For an example, we'll ignore stable ordering but slice according to offset/limit.
	total := len(res)
	if offset > total {
		return []*Todo{}, total
	}
	end := offset + limit
	if limit <= 0 {
		end = total
	}
	if end > total {
		end = total
	}
	return res[offset:end], total
}

func (s *store) get(id int64) (*Todo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.items[id]
	return t, ok
}

func (s *store) create(title string) *Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.next
	s.next++
	now := time.Now().UTC()
	t := &Todo{ID: id, Title: title, CreatedAt: now, UpdatedAt: now}
	s.items[id] = t
	return t
}

func (s *store) replace(id int64, t *Todo) (*Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.items[id]
	if !ok {
		return nil, errors.New("not found")
	}
	now := time.Now().UTC()
	cur.Title = t.Title
	cur.Completed = t.Completed
	cur.UpdatedAt = now
	return cur, nil
}

func (s *store) patch(id int64, fields map[string]any) (*Todo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.items[id]
	if !ok {
		return nil, errors.New("not found")
	}
	if v, ok := fields["title"].(string); ok {
		cur.Title = v
	}
	if v, ok := fields["completed"].(bool); ok {
		cur.Completed = v
	}
	cur.UpdatedAt = time.Now().UTC()
	return cur, nil
}

func (s *store) delete(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return false
	}
	delete(s.items, id)
	return true
}

func main() {
	logger := slog.Default()
	st := newStore()

	r := quokka.New()
	r.Use(
		quokka.Recover(logger),
		quokka.Logger(quokka.LoggerConfig{Logger: logger}),
		quokka.CORS(quokka.DefaultCORSConfig()),
		quokka.SecurityHeaders(quokka.DefaultSecurityHeadersConfig()),
	)

	// Health
	r.GET("/health", func(c *quokka.Context) { c.JSON(http.StatusOK, map[string]string{"status": "ok"}) })

	// Static example (serves this repo's LICENSE under /static/LICENSE if present)
	r.File("/static/LICENSE", "LICENSE")

	// Todos group
	api := r.Group("/api")

	// OPTIONS handlers advertise allowed methods
	allow := func(methods string) quokka.Handler {
		return func(c *quokka.Context) {
			c.SetHeader("Allow", methods)
			c.Status(http.StatusNoContent)
		}
	}
	api.OPTIONS("/todos", allow("OPTIONS, GET, POST"))
	api.OPTIONS("/todos/:id", allow("OPTIONS, GET, PUT, PATCH, DELETE, HEAD"))

	// List with pagination (?offset=&limit=)
	api.GET("/todos", func(c *quokka.Context) {
		offset, _ := strconv.Atoi(c.Query("offset"))
		limit, _ := strconv.Atoi(c.Query("limit"))
		items, total := st.list(offset, limit)
		c.JSON(http.StatusOK, map[string]any{"items": items, "total": total, "offset": offset, "limit": limit})
	})

	// Create
	type createReq struct {
		Title string `json:"title"`
	}
	api.POST("/todos", func(c *quokka.Context) {
		var req createReq
		if err := c.BindJSON(&req); err != nil || strings.TrimSpace(req.Title) == "" {
			c.JSON(http.StatusBadRequest, quokka.ErrorResponse{Error: "invalid_request", Message: "title is required"})
			return
		}
		t := st.create(req.Title)
		c.JSON(http.StatusCreated, t)
	})

	// Retrieve
	api.GET("/todos/:id", func(c *quokka.Context) {
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		if t, ok := st.get(id); ok {
			c.JSON(http.StatusOK, t)
			return
		}
		c.JSON(http.StatusNotFound, quokka.ErrorResponse{Error: "not_found", Message: "todo not found"})
	})

	// HEAD existence
	api.HEAD("/todos/:id", func(c *quokka.Context) {
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		if _, ok := st.get(id); ok {
			c.NoContent()
			return
		}
		c.Status(http.StatusNotFound)
	})

	// PUT replace
	api.PUT("/todos/:id", func(c *quokka.Context) {
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		var body Todo
		if err := c.BindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, quokka.ErrorResponse{Error: "invalid_json"})
			return
		}
		body.ID = id // enforce path id
		if t, err := st.replace(id, &body); err == nil {
			c.JSON(http.StatusOK, t)
		} else {
			c.JSON(http.StatusNotFound, quokka.ErrorResponse{Error: "not_found", Message: "todo not found"})
		}
	})

	// PATCH partial
	api.PATCH("/todos/:id", func(c *quokka.Context) {
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		var fields map[string]any
		if err := c.BindJSON(&fields); err != nil {
			c.JSON(http.StatusBadRequest, quokka.ErrorResponse{Error: "invalid_json"})
			return
		}
		if t, err := st.patch(id, fields); err == nil {
			c.JSON(http.StatusOK, t)
		} else {
			c.JSON(http.StatusNotFound, quokka.ErrorResponse{Error: "not_found", Message: "todo not found"})
		}
	})

	// DELETE
	api.DELETE("/todos/:id", func(c *quokka.Context) {
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		if ok := st.delete(id); ok {
			c.NoContent()
			return
		}
		c.Status(http.StatusNotFound)
	})

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	srv := quokka.NewServer(quokka.ServerConfig{Addr: addr}, r, logger)
	if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server error", slog.Any("err", err))
		os.Exit(1)
	}
}
